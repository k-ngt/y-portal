package web

import "embed"

//go:embed static/* templates/*
var Content embed.FS
