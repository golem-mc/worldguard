package worldguardplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/google/uuid"
)

// mojangLookupURL is Mojang's bulk name to UUID lookup, at most ten names
// a request. WorldGuard: HttpRepositoryService.forMinecraft
var mojangLookupURL = "https://api.minecraftservices.com/minecraft/profile/lookup/bulk/byname"

var httpClient = &http.Client{Timeout: 10 * time.Second}

// lookupMojang answers the UUIDs of names, by lower-case name, from
// Mojang's profile service.
func lookupMojang(names []string) (map[string]plugin.UUID, error) {
	out := map[string]plugin.UUID{}
	for start := 0; start < len(names); start += 10 {
		body, _ := json.Marshal(names[start:min(start+10, len(names))])
		resp, err := httpClient.Post(mojangLookupURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		var found []struct{ ID, Name string }
		err = json.NewDecoder(resp.Body).Decode(&found)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("profile lookup: %s", resp.Status)
		}
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			if id, err := uuid.Parse(p.ID); err == nil {
				out[strings.ToLower(p.Name)] = id
			}
		}
	}
	return out, nil
}

// locatorPolicy is how a domain input names a player. WorldGuard:
// DomainInputResolver.UserLocatorPolicy
type locatorPolicy uint8

const (
	uuidOnly locatorPolicy = iota
	nameOnly
	uuidAndName
)

var groupInput = regexp.MustCompile(`(?i)^g:(.+)$`)

// maxNameLength is the longest player name; a longer one is not looked up.
const maxNameLength = 16

// domainInput is a list of players and groups typed in a command, resolved
// in two steps: names the server knows on the tick goroutine, the others
// through Mojang off it. WorldGuard: DomainInputResolver
type domainInput struct {
	domain *worldguard.Domain
	// query are the lower-case names left to look up.
	query []string
}

// parseUUIDInput reads "uuid:<uuid>" or a UUID with or without dashes.
func parseUUIDInput(s string) (plugin.UUID, bool) {
	s = strings.TrimPrefix(s, "uuid:")
	if len(s) != 32 && len(s) != 36 {
		return plugin.UUID{}, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}

// resolveLocal starts resolving input on the tick goroutine.
func (wg *WorldGuard) resolveLocal(input []string, policy locatorPolicy) *domainInput {
	d := &domainInput{domain: worldguard.NewDomain()}
	for _, s := range input {
		if m := groupInput.FindStringSubmatch(s); m != nil {
			d.domain.AddGroup(m[1])
			continue
		}
		if id, ok := parseUUIDInput(s); ok {
			d.domain.AddPlayer(id)
			continue
		}
		switch policy {
		case nameOnly:
			d.domain.AddPlayerName(s)
			continue
		case uuidAndName:
			d.domain.AddPlayerName(s)
		}
		if len(s) <= maxNameLength {
			if p, ok := wg.Server.OfflinePlayerNamed(s); ok {
				d.domain.AddPlayer(p.UUID())
				continue
			}
		}
		d.query = append(d.query, strings.ToLower(s))
	}
	return d
}

// resolve finishes the lookup off the tick goroutine.
func (d *domainInput) resolve() (*worldguard.Domain, error) {
	var valid []string
	for _, n := range d.query {
		if len(n) <= maxNameLength {
			valid = append(valid, n)
		}
	}
	left := d.query
	if len(valid) > 0 {
		found, err := lookupMojang(valid)
		if err != nil {
			return nil, errorf("The UUID lookup service failed so the names entered could not be turned into UUIDs")
		}
		left = nil
		for _, n := range d.query {
			if id, ok := found[n]; ok {
				d.domain.AddPlayer(id)
			} else {
				left = append(left, n)
			}
		}
	}
	if len(left) > 0 {
		return nil, errorf("Unable to resolve the names %s", strings.Join(left, ", "))
	}
	return d.domain, nil
}

// lastKnownName answers the name the server last saw a UUID under.
func (wg *WorldGuard) lastKnownName(id plugin.UUID) (string, bool) {
	if p, ok := wg.Server.OfflinePlayer(id); ok && p.Name() != "" {
		return p.Name(), true
	}
	return "", false
}
