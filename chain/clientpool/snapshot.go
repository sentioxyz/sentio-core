package clientpool

import (
	"sentioxyz/sentio-core/common/pool"
	"sentioxyz/sentio-core/common/utils"
	"sort"
	"time"
)

type state string

const (
	stateDisabled  state = "Disabled"
	stateNotReady  state = "NotReady"
	stateBroken    state = "Broken"
	stateRecovered state = "Recovered"
	stateOK        state = "OK"
)

var stateIndex = map[state]int{
	stateOK:        0,
	stateRecovered: 1,
	stateBroken:    2,
	stateNotReady:  3,
	stateDisabled:  4,
}

func (p *ClientPool[CONFIG, CLIENT]) _clientState(
	entName string,
	ent pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]],
	now time.Time,
) state {
	if !ent.Enable {
		return stateDisabled
	}
	if !ent.Status.Initialized {
		return stateNotReady
	}
	if extra, has := p.entryExtra[entName]; has && extra.ban != nil {
		if extra.ban.enable(now) {
			return stateBroken
		} else {
			return stateRecovered
		}
	}
	return stateOK
}

func (p *ClientPool[CONFIG, CLIENT]) Snapshot() any {
	p.mu.Lock()
	defer p.mu.Unlock()
	entries, ps, psIndex := p.pool.Fetch(
		func(_ string, _ pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]], _ poolStatus) bool {
			return true
		})
	type client struct {
		ent        pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]]
		name       string
		publicName string
		state      state
	}
	clients := make([]client, 0, len(entries))
	now := time.Now()
	for entName, ent := range entries {
		var publicName string
		if ent.Status.Initialized {
			publicName = ent.Status.Client.GetName()
		}
		clients = append(clients, client{
			ent:        ent,
			name:       entName,
			publicName: publicName,
			state:      p._clientState(entName, ent, now),
		})
	}
	sort.Slice(clients, func(i, j int) bool {
		if clients[i].ent.Config.Priority == clients[j].ent.Config.Priority {
			return stateIndex[clients[i].state] < stateIndex[clients[j].state]
		}
		return clients[i].ent.Config.Priority < clients[j].ent.Config.Priority
	})
	downgrade := map[string]any{
		"statistic": p.statDowngrade.Snapshot(),
	}
	if len(p.configPriorities) > 0 {
		downgrade["currentPriority"] = p.configPriorities[p.priorityCursor]
	}
	return map[string]any{
		"name": p.pool.Name(),
		"config": map[string]any{
			"current":    p.config,
			"version":    p.configVersion,
			"updateAt":   p.configUpdateAt.String(),
			"priorities": p.configPriorities,
		},
		"status":      ps.Snapshot(),
		"statusIndex": psIndex,
		"downgrade":   downgrade,
		"clients": utils.MapSliceNoError(clients, func(cli client) any {
			stateDetail := map[string]any{
				"enable": cli.ent.Enable,
			}
			extra, has := p.entryExtra[cli.name]
			if has && !extra.tags.Empty() {
				stateDetail["tags"] = extra.tags.DumpValues()
			}
			if has && extra.ban != nil {
				stateDetail["ban"] = extra.ban.String()
			}
			if has && extra.active != nil {
				stateDetail["lastActive"] = extra.active.String()
			}
			return map[string]any{
				"name":        cli.name,
				"publicName":  cli.publicName,
				"config":      cli.ent.Config,
				"status":      cli.ent.Status.Snapshot(),
				"state":       cli.state,
				"stateDetail": stateDetail,
			}
		}),
		"consumers": map[string]any{
			"total":        p.consumerCounter,
			"currentCount": len(p.consumer),
			"current":      utils.MapMapNoError(p.consumer, consumer.Snapshot),
		},
	}
}
