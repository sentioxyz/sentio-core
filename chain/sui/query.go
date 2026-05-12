package sui

import (
	"bytes"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/log"
)

func (filter EventFilter) Check(event types.Event) bool {
	switch filter.Op {
	case EventFilterAnd:
		if filter.Left == nil || filter.Right == nil {
			return false
		}
		return filter.Left.Check(event) && filter.Right.Check(event)
	case EventFilterOr:
		if filter.Left == nil || filter.Right == nil {
			return true
		}
		return filter.Left.Check(event) || filter.Right.Check(event)
	default:
		if filter.PackageID != nil && event.PackageID != *filter.PackageID {
			return false
		}
		if filter.TransactionModule != "" && event.TransactionModule != filter.TransactionModule {
			return false
		}
		if filter.Sender != "" && event.Sender != filter.Sender {
			return false
		}
		if filter.Type != nil && !filter.Type.Include(event.Type) {
			return false
		}
		return true
	}
}

func (filter EventFilter) Filter(events []types.Event) []types.Event {
	var result []types.Event
	for _, event := range events {
		if filter.Check(event) {
			result = append(result, event)
		}
	}
	return result
}

func (filter MoveCallFilter) Check(call types.MoveCall) bool {
	if filter.Package != nil && call.Package != *filter.Package {
		return false
	}
	if filter.Module != "" && call.Module != filter.Module {
		return false
	}
	if filter.Function != "" && call.Function != filter.Function {
		return false
	}
	return true
}

func (query TransactionQuery) CheckAndTrim(r *types.TransactionResponseV1) bool {
	if query.FromSequenceNumber > r.Checkpoint.Uint64() || query.ToSequenceNumber < r.Checkpoint.Uint64() {
		return false
	}
	if r.Transaction == nil {
		return query.IncludeFailed && query.Kind == "" && query.MoveCallFilter == nil
	}
	if r.Effects.Status.Status != types.TransactionStatusSuccess && !query.IncludeFailed {
		return false
	}
	if query.Sender != nil && r.Transaction.Data.V1.Sender != *query.Sender {
		return false
	}
	if query.BalanceChange != nil && query.BalanceChange.AddressOwner != nil {
		found := false
		for _, bc := range r.BalanceChanges {
			if bc.Owner == nil || bc.Owner.ObjectOwnerInternal == nil || bc.Owner.AddressOwner == nil {
				continue
			}
			if *bc.Owner.AddressOwner == *query.BalanceChange.AddressOwner {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	tx := r.Transaction.Data.V1.Kind
	txKind := tx.Kind()
	if query.Kind != "" && query.Kind != txKind {
		return false
	}
	if query.Kind == txKind && tx.ProgrammableTransaction != nil && query.MoveCallFilter != nil {
		commands := tx.ProgrammableTransaction.Commands
		hasMatch := false
		for i := range commands {
			if commands[i].MoveCall == nil {
				continue
			}
			if query.MoveCallFilter.Check(*commands[i].MoveCall) {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			return false
		}
	}
	if len(query.MultiSigPublicKeyPrefix) > 0 {
		b := query.MultiSigPublicKeyPrefix
		// Check if the transaction is a multisig transaction.
		if len(r.Transaction.TxSignatures) != 1 {
			return false
		}
		s := r.Transaction.TxSignatures[0]
		if !types.IsMultiSigBytes(s) {
			return false
		}
		// Check if the multisig public key matches the prefix.
		sig, err := types.DecodeMultiSigBytes(s)
		if err != nil {
			log.Errorf("failed to decode multisig bytes: %v", err)
			return false
		}
		found := false
		for _, pkMap := range sig.PublicKey.PkMap {
			pk := pkMap.PubKey
			var key []byte
			switch {
			case pk.ED25519 != [32]byte{}:
				key = pk.ED25519[:]
			case pk.Secp256r1 != [33]byte{}:
				key = pk.Secp256r1[:]
			case pk.Secp256k1 != [33]byte{}:
				key = pk.Secp256k1[:]
			}
			if bytes.HasPrefix(key, b) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	if query.EventFilter != nil {
		filteredEvents := query.EventFilter.Filter(r.Events)
		if len(filteredEvents) == 0 {
			return false
		}
		if query.OnlyFilteredEvents {
			r.Events = filteredEvents
		}
	}
	if query.ExcludeEffects {
		r.Effects = nil
	}
	if query.ExcludeInputs {
		r.Transaction = nil
	}
	return true
}
