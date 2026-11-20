package worldguardplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/google/pprof/profile"
)

// pasteBase is EngineHub's paste service. WorldEdit: EngineHubPaste
var pasteBase = "https://paste.enginehub.org"

// pasteClient uploads pastes.
var pasteClient = &http.Client{Timeout: 30 * time.Second}

// uploadPaste stores content on EngineHub's paste service, which first signs
// an upload URL, and returns the URL to view it. WorldEdit: EngineHubPaste
func uploadPaste(content string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, pasteBase+"/signed_paste_v2", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-paste-meta-from", "EngineHub")
	body, status, err := doPaste(req)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("failed to initiate paste upload: %d %s", status, body)
	}
	var signed struct {
		ViewURL   string            `json:"viewUrl"`
		UploadURL string            `json:"uploadUrl"`
		Headers   map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(body, &signed); err != nil {
		return "", err
	}
	up, err := http.NewRequestWithContext(context.Background(), http.MethodPut, signed.UploadURL, strings.NewReader(content))
	if err != nil {
		return "", err
	}
	for k, v := range signed.Headers {
		up.Header.Set(k, v)
	}
	body, status, err = doPaste(up)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return "", fmt.Errorf("failed to upload paste: %d %s", status, body)
	}
	return signed.ViewURL, nil
}

func doPaste(req *http.Request) ([]byte, int, error) {
	resp, err := pasteClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

// pasteTo uploads content in the background and tells s the URL as
// "<prefix><url><suffix>", or why it failed. WorldEdit: ActorCallbackPaste
func pasteTo(wg *WorldGuard, s plugin.CommandSender, content, prefix, suffix string) {
	async(wg, s, "(Please wait... sending paste...)", "Failed to submit paste", func() (string, error) {
		url, err := uploadPaste(content)
		if err != nil {
			return "", cmdError(err.Error())
		}
		return url, nil
	}, func(url string) {
		s.SendMessage(line(
			txt(prefix, "light_purple"),
			txt(url, "aqua").OnClick(plugin.OpenURL, url),
			txt(suffix, "light_purple")))
	})
}

// profileText renders a CPU profile as the text a paste shows: functions by
// their own time.
func profileText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	p, err := profile.Parse(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	idx := len(p.SampleType) - 1
	for i, st := range p.SampleType {
		if st.Type == "cpu" {
			idx = i
		}
	}
	flat := map[string]int64{}
	var total int64
	for _, s := range p.Sample {
		total += s.Value[idx]
		if len(s.Location) == 0 || len(s.Location[0].Line) == 0 {
			continue
		}
		flat[s.Location[0].Line[0].Function.Name] += s.Value[idx]
	}
	names := make([]string, 0, len(flat))
	for n := range flat {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if flat[names[i]] != flat[names[j]] {
			return flat[names[i]] > flat[names[j]]
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "CPU profile: %s of samples over %s\n\n", time.Duration(total), time.Duration(p.DurationNanos))
	for _, n := range names {
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(flat[n]) / float64(total)
		}
		fmt.Fprintf(&b, "%6.2f%% %12s  %s\n", pct, time.Duration(flat[n]), n)
	}
	return b.String(), nil
}
