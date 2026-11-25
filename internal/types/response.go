package types

type RootResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
	Uptime  string `json:"uptime"`
	Version string `json:"version"`
	Workers int    `json:"workers"`
}
