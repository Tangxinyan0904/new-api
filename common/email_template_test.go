package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderVerificationEmailEscapesEveryField(t *testing.T) {
	savedName := SystemName
	SystemName = `Shop <img src=x onerror=alert(1)>`
	t.Cleanup(func() { SystemName = savedName })

	content, err := RenderVerificationEmail(VerificationEmail{
		Preview: "Your code is a3f9c1",
		Title:   "<script>alert(1)</script>",
		Intro:   "Fish & chips",
		Code:    "a3f9c1",
		Hints:   []string{"Valid for 10 minutes"},
		Notice:  "Ignore it",
		Footer:  "Sent by Shop",
	})
	require.NoError(t, err)

	assert.NotContains(t, content, "<script>")
	assert.NotContains(t, content, "<img")
	assert.Contains(t, content, "&lt;script&gt;")
	assert.Contains(t, content, "Fish &amp; chips")
	assert.Contains(t, content, ">a3f9c1</div>", "the code stands alone so one selection copies all of it")
	assert.Contains(t, content, "Valid for 10 minutes")
	assert.NotContains(t, content, "<a ", "a code email has no button")
}

func TestRenderVerificationEmailLinksOnlyToWebAddresses(t *testing.T) {
	link := "https://example.com/user/reset?email=a@example.com&token=abc"
	content, err := RenderVerificationEmail(VerificationEmail{
		Title: "Reset", ActionURL: link, ActionLabel: "Reset password", LinkHint: "Or copy this link:",
	})
	require.NoError(t, err)
	assert.Contains(t, content, `href="https://example.com/user/reset?email=a@example.com&amp;token=abc"`)
	assert.Contains(t, content, ">https://example.com/user/reset?email=a@example.com&amp;token=abc</p>", "the link is also written out")

	unsafe, err := RenderVerificationEmail(VerificationEmail{Title: "Reset", ActionURL: "javascript:alert(1)", ActionLabel: "Reset"})
	require.NoError(t, err)
	assert.False(t, strings.Contains(unsafe, `href="javascript:`), "a script URL never becomes a link")
}
