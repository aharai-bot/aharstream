package utils

import (
	"fmt"
	"mime"
	"strings"
	"time"
)

// HumanSize converts bytes to a human readable string (e.g. 1.46 GB).
func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ShortName truncates long file names for chat messages.
func ShortName(name string, max int) string {
	if name == "" {
		return "unnamed"
	}
	r := []rune(name)
	if len(r) <= max {
		return name
	}
	return string(r[:max-1]) + "…"
}

// FormatUnix renders a unix time as UTC, or "never" for 0.
func FormatUnix(ts int64) string {
	if ts == 0 {
		return "never"
	}
	return time.Unix(ts, 0).UTC().Format("02 Jan 2006 15:04 UTC")
}

// TimeLeft renders a human "in 3h 20m" / "expired" string for an expiry time.
func TimeLeft(ts int64) string {
	if ts == 0 {
		return "never expires"
	}
	d := time.Until(time.Unix(ts, 0))
	if d <= 0 {
		return "expired"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h >= 48:
		return fmt.Sprintf("%dd %dh left", h/24, h%24)
	case h > 0:
		return fmt.Sprintf("%dh %dm left", h, m)
	default:
		return fmt.Sprintf("%dm left", m)
	}
}

// FileKind returns an emoji + label for a mime type.
func FileKind(mime string) string {
	switch {
	case strings.HasPrefix(mime, "video/"):
		return "🎬 Video"
	case strings.HasPrefix(mime, "audio/"):
		return "🎵 Audio"
	case strings.HasPrefix(mime, "image/"):
		return "🖼 Image"
	case strings.Contains(mime, "pdf"):
		return "📕 PDF"
	case strings.Contains(mime, "zip"), strings.Contains(mime, "rar"), strings.Contains(mime, "7z"), strings.Contains(mime, "tar"):
		return "🗜 Archive"
	case strings.HasPrefix(mime, "text/"):
		return "📝 Text"
	default:
		return "📦 File"
	}
}

// Playable reports whether the web player can render the mime type.
func Playable(mime string) bool {
	return strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "audio/") ||
		strings.HasPrefix(mime, "image/") || strings.Contains(mime, "pdf")
}

// DisplayName returns a usable file name even for media without a filename attribute.
// It is for display and Content-Disposition only; hashes keep using the raw name.
func DisplayName(name, mimeType string, id int64) string {
	if name != "" {
		return name
	}
	ext := ""
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	return fmt.Sprintf("file_%d%s", id, ext)
}
