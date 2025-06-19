package util

// Define common ANSI escape code constants
const (
	Reset = "\x1b[0m" // Reset all attributes
	// Regular Colors (Foreground)
	FgBlack   = "\x1b[30m"
	FgRed     = "\x1b[31m"
	FgGreen   = "\x1b[32m"
	FgYellow  = "\x1b[33m"
	FgBlue    = "\x1b[34m"
	FgMagenta = "\x1b[35m"
	FgCyan    = "\x1b[36m"
	FgWhite   = "\x1b[37m"
	// Bright/High-Intensity Colors (Foreground)
	FgBrightBlack   = "\x1b[90m" // Often called Gray
	FgBrightRed     = "\x1b[91m"
	FgBrightGreen   = "\x1b[92m"
	FgBrightYellow  = "\x1b[93m"
	FgBrightBlue    = "\x1b[94m"
	FgBrightMagenta = "\x1b[95m"
	FgBrightCyan    = "\x1b[96m"
	FgBrightWhite   = "\x1b[97m"
	// Background Colors
	BgBlack   = "\x1b[40m"
	BgRed     = "\x1b[41m"
	BgGreen   = "\x1b[42m"
	BgYellow  = "\x1b[43m"
	BgBlue    = "\x1b[44m"
	BgMagenta = "\x1b[45m"
	BgCyan    = "\x1b[46m"
	BgWhite   = "\x1b[47m"
	// Bright/High-Intensity Background Colors
	BgBrightBlack   = "\x1b[100m"
	BgBrightRed     = "\x1b[101m"
	BgBrightGreen   = "\x1b[102m"
	BgBrightYellow  = "\x1b[103m"
	BgBrightBlue    = "\x1b[104m"
	BgBrightMagenta = "\x1b[105m"
	BgBrightCyan    = "\x1b[106m"
	BgBrightWhite   = "\x1b[107m"
	// Styles
	Bold      = "\x1b[1m"
	Dim       = "\x1b[2m" // Dim/Faint
	Italic    = "\x1b[3m" // Italic (not supported by all terminals)
	Underline = "\x1b[4m" // Underline
	Blink     = "\x1b[5m" // Blink (not supported by all terminals)
	Reverse   = "\x1b[7m" // Reverse foreground and background colors
	Hidden    = "\x1b[8m" // Hidden (useful for password input)
)
