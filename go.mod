module github.com/WeidiDeng/ttyd-go

go 1.23.0

require (
	github.com/creack/pty v1.1.24
	github.com/gobwas/httphead v0.1.0
	github.com/gobwas/ws v1.4.0
)

require github.com/gobwas/pool v0.2.1 // indirect

replace github.com/creack/pty => github.com/WeidiDeng/pty v0.0.0-20260312031053-e47170d80e8c
