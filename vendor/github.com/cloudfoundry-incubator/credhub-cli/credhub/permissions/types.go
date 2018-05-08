// CredHub permission types
package permissions

type Permission struct {
	Actor      string   `json:"actor"`
	Operations []string `json:"operations"`
}
