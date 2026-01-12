package workspace

import "errors"

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrChannelNotFound   = errors.New("channel not found")
	ErrDuplicateChannel  = errors.New("channel already exists")
)
