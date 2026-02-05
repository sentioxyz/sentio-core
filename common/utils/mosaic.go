package utils

import (
	"net/url"
	"strings"
)

func mosaic(ch byte, len int) string {
	s := make([]byte, len)
	for i := range s {
		s[i] = ch
	}
	return string(s)
}

func AddURLMosaic(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return raw
	}

	if strings.Contains(u.Host, "sentio") {
		host, port, hasPort := strings.Cut(u.Host, ":")
		sectors := strings.Split(host, ".")
		for i := range sectors {
			if p := strings.Index(sectors[i], "sentio"); p < 0 {
				sectors[i] = mosaic('*', len(sectors[i]))
			} else {
				sectors[i] = mosaic('*', p) + "sentio" + mosaic('*', len(sectors[i])-p-6)
			}
		}
		host = strings.Join(sectors, ".")
		if hasPort {
			host = host + ":" + port
		}
		u.Host = host
	} else {
		// path part
		sectors := strings.Split(u.Path, "/")
		for i, sector := range sectors {
			if len(sector) >= 16 {
				sectors[i] = sector[:len(sector)-8] + mosaic('x', 8)
			}
		}
		u.Path = strings.Join(sectors, "/")
		// query part
		query := make(url.Values)
		for k, vs := range u.Query() {
			for _, v := range vs {
				if len(v) >= 16 {
					v = v[:len(v)-8] + mosaic('x', 8)
				}
				query.Add(k, v)
			}
		}
		u.RawQuery = query.Encode()
		// user part
		if u.User != nil {
			username := u.User.Username()
			password, has := u.User.Password()
			if has {
				u.User = url.UserPassword(mosaic('x', len(username)), mosaic('x', len(password)))
			} else {
				u.User = url.User(mosaic('x', len(username)))
			}
		}
	}
	return u.String()
}

func AddOwnerNameMosaic(raw string) string {
	rl := len(raw)
	if rl <= 2 {
		return raw
	}
	vl := min(3, rl/3)
	return raw[:vl] + mosaic('*', rl-vl*2) + raw[rl-vl:]
}
