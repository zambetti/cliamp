package control

import "testing"

// The control package only defines zero-sized struct message types shared
// between MPRIS and IPC dispatchers. These smoke tests just confirm the
// types are each their own distinct Go type so a type switch can discriminate.
func TestMessageTypesDiscriminate(t *testing.T) {
	cases := []struct {
		name string
		msg  any
		tag  string
	}{
		{"toggle", ToggleMsg{}, "toggle"},
		{"next", NextMsg{}, "next"},
		{"prev", PrevMsg{}, "prev"},
		{"stop", StopMsg{}, "stop"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tag string
			switch c.msg.(type) {
			case ToggleMsg:
				tag = "toggle"
			case NextMsg:
				tag = "next"
			case PrevMsg:
				tag = "prev"
			case StopMsg:
				tag = "stop"
			default:
				tag = "unknown"
			}
			if tag != c.tag {
				t.Errorf("switch(%T) dispatched to %q, want %q", c.msg, tag, c.tag)
			}
		})
	}
}
