package common

import "errors"

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrChannelNotFound   = errors.New("channel not found")
	ErrDuplicateChannel  = errors.New("channel already exists")
	ErrDuplicateRule     = errors.New("identity rule already exists for this channel")
)
