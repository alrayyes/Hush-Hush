// Package api is the composition root's one dependency: an http.Handler
// satisfying api/openapi.yaml. It lives under internal/ because the API this
// service offers is its endpoints, not its Go packages — see go.md's "a
// server keeps everything in internal/ and its commands in cmd/".
package api
