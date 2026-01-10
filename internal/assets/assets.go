package assets

import "embed"

// static contains files served under /static (cats avatars, ui images, etc.)
//
//go:embed static/**
var FS embed.FS
