package synchooks

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/esauvisky/gitgram/internal/gitlab/api"
)

// hookSpec maps the enabled config event classes onto GitLab hook flags.
// Classes: push, pipeline (job + pipeline).
func hookSpec(opts Options) (api.HookSpec, error) {
	spec := api.HookSpec{
		URL:                   opts.WebhookURL,
		Token:                 opts.Secret,
		EnableSSLVerification: strings.HasPrefix(opts.WebhookURL, "https://"),
	}
	classes := make([]string, 0, len(opts.Events))
	for class, on := range opts.Events {
		if on {
			classes = append(classes, class)
		}
	}
	sort.Strings(classes)
	for _, class := range classes {
		switch class {
		case "push":
			spec.PushEvents = true
		case "pipeline":
			spec.JobEvents = true
			spec.PipelineEvents = true
		default:
			return api.HookSpec{}, fmt.Errorf("synchooks: unknown event class %q", class)
		}
	}
	return spec, nil
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
