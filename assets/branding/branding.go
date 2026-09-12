// Package branding embeds Silk IDE's application identity. SDK applications
// opt in explicitly; importing gui/ged alone does not adopt the IDE's branding.
package branding

import _ "embed"

//go:embed silkide-256.png
var logoPNG []byte

// PNG returns an independent copy of the runtime logo. Keeping it in the
// executable makes startup independent of the current directory and writable
// installation paths. Native launcher assets are packaged separately.
func PNG() []byte { return append([]byte(nil), logoPNG...) }
