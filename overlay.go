package bubble_overlay

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Internal ID management. Used during animating to assure that frame messages
// can only be received by progress components that sent them.
var (
	lastID int
	idMtx  sync.Mutex
)

// Return the next ID we should use on the model.
func nextID() int {
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}

// const (
// 	fps              = 60
// 	defaultFrequency = 18.0
// 	defaultDamping   = 1.0
// )

var (
	defaultOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#874BFD")).
				Padding(1, 0).
				BorderTop(true).
				BorderLeft(true).
				BorderRight(true).
				BorderBottom(true)

	defaultBackdropStyle = lipgloss.NewStyle().Background(lipgloss.Color("255")).Faint(true)
)

// FrameMsg indicates that an animation step should occur.
// type FrameMsg struct {
// 	id  int
// 	tag int
// }

// TimeoutMsg indicates that the overlay should be opened or closed
type TimeoutMsg struct {
	id    int
	tag   int
	state bool
}

type Model struct {
	// An identifier to keep us from receiving messages intended for other
	// overlays.
	id int

	// An identifier to keep us from receiving frame messages too quickly.
	tag int

	Overlay       string         // the content to be displayed
	Style         lipgloss.Style // the style of the overlay
	Backdrop      string         // the content to hide behind the overlay
	BackdropStyle lipgloss.Style // the style of the backdrop

	// Width  int // the width of the entire element
	// Height int // the height of the entire element

	Vertical   lipgloss.Position // the vertical alignment of the overlay on the backdrop, defaults to center
	Horizontal lipgloss.Position // the horizontal alignment of the overlay on the backdrop, defaults to center

	OpenTimeout  time.Duration // the duration to keep the overlay open
	CloseTimeout time.Duration // the duration to keep the overlay closed

	MouseHide []tea.MouseEventType // the mouse events to hide the overlay

	opened      bool      // whether the overlay is currently open
	springStart time.Time // the time the overlay animation was started
}

type Option func(*Model)

func WithDefaultOverlayStyle() Option {
	return WithStyle(defaultOverlayStyle)
}

// set the style of the overlay
func WithStyle(style lipgloss.Style) Option {
	return func(m *Model) {
		m.Style = style
	}
}

// set the backdrop of the overlay
func WithBackdrop(backdrop string) Option {
	return func(m *Model) {
		m.Backdrop = backdrop
	}
}

func WithDefaultBackdropStyle() Option {
	return WithBackdropStyle(defaultBackdropStyle)
}

// set the style of the backdrop
func WithBackdropStyle(style lipgloss.Style) Option {
	return func(m *Model) {
		m.BackdropStyle = style
	}
}

// set the timeout for opening/closing the overlay
//  - open: after opening, keep the overlay open for this duration (ie. automatically close)
//  - close: after closing, keep the overlay closed for this duration (ie. automatically open)
func WithTimeout(open time.Duration, close time.Duration) Option {
	return func(m *Model) {
		m.OpenTimeout = open
		m.CloseTimeout = close
	}
}

// set the alignment of the overlay on the backdrop
// defaults to center, center
func WithAlignment(vpos, hpos lipgloss.Position) Option {
	return func(m *Model) {
		m.Vertical = vpos
		m.Horizontal = hpos
	}
}

func WithMouseHide(events ...tea.MouseEventType) Option {
	return func(m *Model) {
		m.MouseHide = append(m.MouseHide, events...)
	}
}

func New(opts ...Option) Model {
	m := Model{
		id:            nextID(),
		Style:         defaultOverlayStyle,
		BackdropStyle: defaultBackdropStyle,
		Vertical:      lipgloss.Center,
		Horizontal:    lipgloss.Center,
	}

	// if !m.springCustomized {
	// 	m.SetSpringOptions(defaultFrequency, defaultDamping)
	// }

	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Open() tea.Cmd {
	return m.open(true)
}

func (m *Model) open(withTimeout bool) tea.Cmd {
	m.opened = true
	m.springStart = time.Now()

	// register message to fire at the end of openTImeout
	if withTimeout && m.OpenTimeout != 0 {
		m.tag++
		return m.nextTimeout(m.OpenTimeout)
	}
	return nil
}

func (m *Model) Close() tea.Cmd {
	return m.close(true)
}

func (m *Model) close(withTimeout bool) tea.Cmd {
	m.opened = false
	m.springStart = time.Now()

	// register message to fire at the end of closeTimeout
	if withTimeout && m.CloseTimeout != 0 {
		m.tag++
		return m.nextTimeout(m.CloseTimeout)
	}

	return nil
}

func (m *Model) nextTimeout(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return TimeoutMsg{id: m.id, tag: m.tag, state: m.opened}
	})
}

func (m *Model) Render(overlay, backdrop string) string {
	if !m.opened {
		return backdrop
	}

	bw, bh := lipgloss.Size(backdrop)
	ow, oh := lipgloss.Size(overlay)
	// calculate the position of the overlay
	//  (backdropWidth - overlayWidth) * horizontalAlignment = leftOffset
	leftOffset := int(float64(bw-ow) * float64(m.Horizontal)) // 0 <= leftOffset <= backdropWidth - overlayWidth
	//  (backdropHeight - overlayHeight) * verticalAlignment = topOffset
	topOffset := int(float64(bh-oh) * float64(m.Vertical)) // 0 <= topOffset <= backdropHeight - overlayHeight

	// splice the overlay into the backdrop at the calculated offsets
	blines := strings.Split(backdrop, "\n")
	olines := strings.Split(overlay, "\n")
	for i := topOffset; i < topOffset+len(olines); i++ {
		// replace the line starting with line[yOffset] at position xOffset to overlayWidth
		line := blines[i]

		left := lipgloss.NewStyle().MaxWidth(leftOffset).Render(line)
		leftSub := lipgloss.NewStyle().MaxWidth(leftOffset + ow).Render(line)

		// right = line - leftSub
		// right := strings.Replace(line, leftSub, "", 1)
		right := string([]byte(line)[len([]byte(leftSub)):])

		idx := i - topOffset
		blines[i] = left + olines[idx] + right
	}

	return strings.Join(blines, "\n")
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	// case FrameMsg:
	// 	// animate spring

	case TimeoutMsg:
		// open or close the overlay
		if msg.id == m.id && msg.tag == m.tag {
			if msg.state {
				// opened --> closed
				return m, m.close(false)
			} else {
				// closed --> opened
				return m, m.open(false)
			}
		}
		return m, nil
	case tea.MouseMsg:
		if len(m.MouseHide) > 0 {
			if m.mouseHideHas(msg.Type) {
				return m, m.close(false)
			}
		}
	default:
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	return m.Render(m.Overlay, m.Backdrop)
}

func (m Model) mouseHideHas(eventType tea.MouseEventType) bool {
	for _, a := range m.MouseHide {
		if a == eventType {
			return true
		}
	}
	return false
}
