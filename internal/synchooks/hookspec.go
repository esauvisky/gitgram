package synchooks

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// hookSpec is the hook the bot needs: push, job, pipeline and merge
// request events, every other event off.
func hookSpec(opts Options) api.HookSpec {
	return api.HookSpec{
		URL:                   opts.WebhookURL,
		Token:                 opts.Secret,
		EnableSSLVerification: strings.HasPrefix(opts.WebhookURL, "https://"),
		PushEvents:            true,
		JobEvents:             true,
		PipelineEvents:        true,
		MergeRequestsEvents:   true,
	}
}

// diffSpec lists the fields (by JSON name) whose value differs between the
// hook read back from GitLab and the wanted spec, as "name: have→want".
// Token is skipped because GitLab never returns it.
func diffSpec(have, want api.HookSpec) []string {
	var diffs []string
	t := reflect.TypeOf(have)
	hv, wv := reflect.ValueOf(have), reflect.ValueOf(want)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "Token" {
			continue
		}
		h, w := hv.Field(i).Interface(), wv.Field(i).Interface()
		if h != w {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			diffs = append(diffs, fmt.Sprintf("%s: %v→%v", name, h, w))
		}
	}
	return diffs
}
