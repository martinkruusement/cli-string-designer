package main

import (
    "fmt"
    "github.com/mattn/go-runewidth"
    "github.com/nsf/termbox-go"
    "log"
    "os"
    "strconv"
    "strings"
)

const (
    colorsPerRow = 8
    blockWidth   = 4
)

// Cell keeps track of what we drew at each position (for final ANSI dump)
type Cell struct {
    Ch rune
    Fg termbox.Attribute
    Bg termbox.Attribute
}

// Word holds the data for a "clickable" word in our preview
type Word struct {
    Text string
    Fg   termbox.Attribute
    Bg   termbox.Attribute
    X    int
    Y    int
    W    int
}

var screenW = 120
var screenH = 50
var cells = make([][]Cell, screenH)
var maxUsedX, maxUsedY int

// State
var selectedBackground termbox.Attribute = termbox.ColorDefault
var selectedForeground termbox.Attribute = termbox.ColorDefault

// Index of the currently selected word (-1 means none)
var selectedWord int = -1

// Colorable sections from CLI
var words []Word

func drawWord(x, y int, text string, fg, bg termbox.Attribute) int {
    runes := []rune(text)
    for _, r := range runes {
        w := runewidth.RuneWidth(r)
        setCell(x, y, r, fg, bg)
        // For wide runes, fill the extra cell(s) with spaces
        for i := 1; i < w; i++ {
            setCell(x+i, y, ' ', fg, bg)
        }
        x += w
    }
    return x
}

func main() {
    args := os.Args[1:]

    for y := 0; y < screenH; y++ {
        row := make([]Cell, screenW)
        for x := 0; x < screenW; x++ {
            row[x] = Cell{Ch: ' ', Fg: termbox.ColorDefault, Bg: termbox.ColorDefault}
        }
        cells[y] = row
    }

    // Initialize termbox
    err := termbox.Init()
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        termbox.Close()
        dumpScreenAsANSI()
    }()

    termbox.SetInputMode(termbox.InputEsc | termbox.InputMouse)
    termbox.SetOutputMode(termbox.Output256)

    // Create Word objects from CLI arguments
    for _, arg := range args {
        w := Word{
            Text: arg,
            Fg:   termbox.ColorDefault,   
            Bg:   termbox.ColorDefault, 
        }
        words = append(words, w)
    }

    redrawAll()

    // Main event loop
    for {
        ev := termbox.PollEvent()
        switch ev.Type {
        case termbox.EventKey:
            if ev.Key == termbox.KeyEsc ||
                ev.Key == termbox.KeyCtrlC ||
                ev.Key == termbox.KeyEnter {
                return
            }

        case termbox.EventMouse:
            if ev.Key == termbox.MouseLeft {
                handleMouseClick(ev.MouseX, ev.MouseY)
            }

        case termbox.EventError:
            log.Printf("Error: %v\n", ev.Err)
            return
        }
    }
}

// handleMouseClick decides if the user clicked on the left block (bg),
// right block (fg), or on a word
func handleMouseClick(mx, my int) {
    // Each block is 32 columns wide: 4 chars per color, 8 colors per row
    // The color area is 256/8=32 rows high => y<32
    bgMaxRow := 256 / colorsPerRow // 32

    // Background region:
    if mx < 32 && my < bgMaxRow {
        colIndex := my*colorsPerRow + (mx / blockWidth)
        if colIndex >= 0 && colIndex < 256 {
            // Shift +1 to store in termbox so that "0" => attribute(1) => black
            clickedColor := termbox.Attribute(colIndex + 1)
            if selectedWord >= 0 && selectedWord < len(words) {
                words[selectedWord].Bg = clickedColor
            } else {
                selectedBackground = clickedColor
            }
            redrawAll()
            return
        }
    }

    // Foreground region:
    if mx >= 32 && mx < 64 && my < bgMaxRow {
        colIndex := my*colorsPerRow + (mx-32)/blockWidth
        if colIndex >= 0 && colIndex < 256 {
            // Also shift +1 here so that "0" => attribute(1) => black
            clickedColor := termbox.Attribute(colIndex + 1)
            if selectedWord >= 0 && selectedWord < len(words) {
                words[selectedWord].Fg = clickedColor
            } else {
                selectedForeground = clickedColor
            }
            redrawAll()
            return
        }
    }

    // Check if it was on a word
    for i := range words {
        w := &words[i]
        if my == w.Y {
            if mx >= w.X && mx < (w.X+w.W) {
                selectedWord = i
                redrawAll()
                return
            }
        }
    }

    // Otherwise, deselect
    selectedWord = -1
    redrawAll()
}

func redrawAll() {
    termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

    // Left side: background color blocks
    for i := 0; i < 256; i++ {
        x := (i % colorsPerRow) * blockWidth
        y := i / colorsPerRow
        // We'll set the BG to (i+1) so that "0" => black
        bg := termbox.Attribute(i + 1)
        fg := selectedForeground

        numStr := strconv.Itoa(i)
        for len(numStr) < 3 {
            numStr = " " + numStr
        }
        for dx, r := range numStr {
            setCell(x+dx, y, r, fg, bg)
        }
    }

    // Right side: foreground color blocks
    for i := 0; i < 256; i++ {
        x := (i % colorsPerRow)*blockWidth + 32
        y := i / colorsPerRow
        // We'll set the FG to (i+1) so that "0" => black
        fg := termbox.Attribute(i + 1)
        bg := selectedBackground

        numStr := strconv.Itoa(i)
        for len(numStr) < 3 {
            numStr = " " + numStr
        }
        for dx, r := range numStr {
            setCell(x+dx, y, r, fg, bg)
        }
    }

    // Place the words below the color blocks
    wordY := 32 + 1
    curX := 1
    for i := range words {
        w := &words[i]
        w.X = curX
        w.Y = wordY
        w.W = len(w.Text)

        displayFg, displayBg := w.Fg, w.Bg
        if i == selectedWord {
            // highlight
            if displayBg == termbox.ColorDefault {
                displayBg = termbox.ColorBlue
            } else {
                displayBg |= termbox.AttrBold
            }
        }
        curX = drawWord(curX, wordY, w.Text, displayFg, displayBg)
        curX++
    }

    termbox.Flush()
}

func setCell(x, y int, ch rune, fg, bg termbox.Attribute) {
    if x < 0 || x >= screenW || y < 0 || y >= screenH {
        return
    }
    termbox.SetCell(x, y, ch, fg, bg)
    cells[y][x] = Cell{Ch: ch, Fg: fg, Bg: bg}
    if x > maxUsedX {
        maxUsedX = x
    }
    if y > maxUsedY {
        maxUsedY = y
    }
}

// dumpScreenAsANSI prints the final "screen" plus a snippet
func dumpScreenAsANSI() {
    prevFg := termbox.ColorDefault
    prevBg := termbox.ColorDefault

    for y := 0; y <= maxUsedY; y++ {
        var line strings.Builder
        prevFg = termbox.ColorDefault
        prevBg = termbox.ColorDefault

        for x := 0; x <= maxUsedX; x++ {
            c := cells[y][x]
            if c.Fg != prevFg {
                line.WriteString(toANSI256(c.Fg, true))
                prevFg = c.Fg
            }
            if c.Bg != prevBg {
                line.WriteString(toANSI256(c.Bg, false))
                prevBg = c.Bg
            }
            line.WriteRune(c.Ch)
        }
        // Reset colors at end of line
        line.WriteString("\x1b[0m\n")
        fmt.Print(line.String())
    }

    // After printing the screen, print a snippet
    printSnippet()

    // Final reset
    fmt.Print("\x1b[0m")
}

// Convert termbox.Attribute (1..256) to an ANSI 256-color code
// If attr == 0 => "default" in termbox. But we used +1 offsets,
// so we convert back to 0..255 by subtracting 1.
func toANSI256(attr termbox.Attribute, isFg bool) string {
    if attr == termbox.ColorDefault {
        if isFg {
            return "\x1b[39m"
        }
        return "\x1b[49m"
    }
    // Convert 1..256 => 0..255 by subtracting 1
    c := int(attr) - 1
    if c < 0 {
        c = 0
    }
    if c > 255 {
        c = 255
    }
    if isFg {
        return fmt.Sprintf("\x1b[38;5;%dm", c)
    }
    return fmt.Sprintf("\x1b[48;5;%dm", c)
}

// printSnippet shows how to reproduce each word's FG/BG
func printSnippet() {
    var snippet strings.Builder
    snippet.WriteString("\n\n echo \"")
    for _, w := range words {
        // Convert termbox.Attribute => 0..255
        fgAttr := int(w.Fg) - 1
        bgAttr := int(w.Bg) - 1

        if w.Fg != termbox.ColorDefault {
            if fgAttr < 0 {
                fgAttr = 0
            } else if fgAttr > 255 {
                fgAttr = 255
            }
            snippet.WriteString(fmt.Sprintf("${C%d}", fgAttr))
        }
        if w.Bg != termbox.ColorDefault {
            if bgAttr < 0 {
                bgAttr = 0
            } else if bgAttr > 255 {
                bgAttr = 255
            }
            snippet.WriteString(fmt.Sprintf("${B%d}", bgAttr))
        }

        snippet.WriteString(w.Text + " ")
    }
    snippet.WriteString("${RESET}\"\n\n")
    fmt.Print(snippet.String())
}
