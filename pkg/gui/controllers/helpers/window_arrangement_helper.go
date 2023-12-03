package helpers

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazycore/pkg/boxlayout"
	"github.com/jesseduffield/lazygit/pkg/gui/context"
	"github.com/jesseduffield/lazygit/pkg/gui/types"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/mattn/go-runewidth"
	"golang.org/x/exp/slices"
)

// In this file we use the boxlayout package, along with knowledge about the app's state,
// to arrange the windows (i.e. panels) on the screen.

type WindowArrangementHelper struct {
	c               *HelperCommon
	windowHelper    *WindowHelper
	modeHelper      *ModeHelper
	appStatusHelper *AppStatusHelper
}

func NewWindowArrangementHelper(
	c *HelperCommon,
	windowHelper *WindowHelper,
	modeHelper *ModeHelper,
	appStatusHelper *AppStatusHelper,
) *WindowArrangementHelper {
	return &WindowArrangementHelper{
		c:               c,
		windowHelper:    windowHelper,
		modeHelper:      modeHelper,
		appStatusHelper: appStatusHelper,
	}
}

func (self *WindowArrangementHelper) shouldUsePortraitMode(width, height int) bool {
	switch self.c.UserConfig.Gui.PortraitMode {
	case "never":
		return false
	case "always":
		return true
	default: // "auto" or any garbage values in PortraitMode value
		return width <= 84 && height > 45
	}
}

func (self *WindowArrangementHelper) GetWindowDimensions(informationStr string, appStatus string) map[string]boxlayout.Dimensions {
	width, height := self.c.GocuiGui().Size()

	sideSectionWeight, mainSectionWeight := self.getMidSectionWeights()

	sidePanelsDirection := boxlayout.COLUMN
	if self.shouldUsePortraitMode(width, height) {
		sidePanelsDirection = boxlayout.ROW
	}

	mainPanelsDirection := boxlayout.ROW
	if self.splitMainPanelSideBySide() {
		mainPanelsDirection = boxlayout.COLUMN
	}

	extrasWindowSize := self.getExtrasWindowSize(height)

	self.c.Modes().Filtering.Active()

	showInfoSection := self.c.UserConfig.Gui.ShowBottomLine ||
		self.c.State().GetRepoState().InSearchPrompt() ||
		self.modeHelper.IsAnyModeActive() ||
		self.appStatusHelper.HasStatus()
	infoSectionSize := 0
	if showInfoSection {
		infoSectionSize = 1
	}

	root := &boxlayout.Box{
		Direction: boxlayout.ROW,
		Children: []*boxlayout.Box{
			{
				Direction: sidePanelsDirection,
				Weight:    1,
				Children: []*boxlayout.Box{
					{
						Direction:           boxlayout.ROW,
						Weight:              sideSectionWeight,
						ConditionalChildren: self.sidePanelChildren,
					},
					{
						Direction: boxlayout.ROW,
						Weight:    mainSectionWeight,
						Children: []*boxlayout.Box{
							{
								Direction: mainPanelsDirection,
								Children:  self.mainSectionChildren(),
								Weight:    1,
							},
							{
								Window: "extras",
								Size:   extrasWindowSize,
							},
						},
					},
				},
			},
			{
				Direction: boxlayout.COLUMN,
				Size:      infoSectionSize,
				Children:  self.infoSectionChildren(informationStr, appStatus),
			},
		},
	}

	layerOneWindows := boxlayout.ArrangeWindows(root, 0, 0, width, height)
	limitWindows := boxlayout.ArrangeWindows(&boxlayout.Box{Window: "limit"}, 0, 0, width, height)

	return MergeMaps(layerOneWindows, limitWindows)
}

func MergeMaps[K comparable, V any](maps ...map[K]V) map[K]V {
	result := map[K]V{}
	for _, currMap := range maps {
		for key, value := range currMap {
			result[key] = value
		}
	}

	return result
}

func (self *WindowArrangementHelper) mainSectionChildren() []*boxlayout.Box {
	currentWindow := self.windowHelper.CurrentWindow()

	// if we're not in split mode we can just show the one main panel. Likewise if
	// the main panel is focused and we're in full-screen mode
	if !self.c.State().GetRepoState().GetSplitMainPanel() || (self.c.State().GetRepoState().GetScreenMode() == types.SCREEN_FULL && currentWindow == "main") {
		return []*boxlayout.Box{
			{
				Window: "main",
				Weight: 1,
			},
		}
	}

	return []*boxlayout.Box{
		{
			Window: "main",
			Weight: 1,
		},
		{
			Window: "secondary",
			Weight: 1,
		},
	}
}

func (self *WindowArrangementHelper) getMidSectionWeights() (int, int) {
	currentWindow := self.windowHelper.CurrentWindow()

	// we originally specified this as a ratio i.e. .20 would correspond to a weight of 1 against 4
	sidePanelWidthRatio := self.c.UserConfig.Gui.SidePanelWidth
	// we could make this better by creating ratios like 2:3 rather than always 1:something
	mainSectionWeight := int(1/sidePanelWidthRatio) - 1
	sideSectionWeight := 1

	if self.splitMainPanelSideBySide() {
		mainSectionWeight = 5 // need to shrink side panel to make way for main panels if side-by-side
	}

	screenMode := self.c.State().GetRepoState().GetScreenMode()

	if currentWindow == "main" {
		if screenMode == types.SCREEN_HALF || screenMode == types.SCREEN_FULL {
			sideSectionWeight = 0
		}
	} else {
		if screenMode == types.SCREEN_HALF {
			mainSectionWeight = 1
		} else if screenMode == types.SCREEN_FULL {
			mainSectionWeight = 0
		}
	}

	return sideSectionWeight, mainSectionWeight
}

func (self *WindowArrangementHelper) infoSectionChildren(informationStr string, appStatus string) []*boxlayout.Box {
	if self.c.State().GetRepoState().InSearchPrompt() {
		var prefix string
		if self.c.State().GetRepoState().GetSearchState().SearchType() == types.SearchTypeSearch {
			prefix = self.c.Tr.SearchPrefix
		} else {
			prefix = self.c.Tr.FilterPrefix
		}
		return []*boxlayout.Box{
			{
				Window: "searchPrefix",
				Size:   runewidth.StringWidth(prefix),
			},
			{
				Window: "search",
				Weight: 1,
			},
		}
	}

	statusSpacerPrefix := "statusSpacer"
	spacerBoxIndex := 0
	maxSpacerBoxIndex := 4 // See pkg/gui/types/views.go
	// Returns a box with size 1 to be used as padding before, between, or after views
	spacerBox := func() *boxlayout.Box {
		spacerBoxIndex++

		if spacerBoxIndex > maxSpacerBoxIndex {
			panic("Too many spacer boxes")
		}

		return &boxlayout.Box{Window: fmt.Sprintf("%s%d", statusSpacerPrefix, spacerBoxIndex), Size: 1}
	}

	// Returns a box with weight 1 to be used as flexible padding before, between, or after views
	flexibleSpacerBox := func() *boxlayout.Box {
		spacerBoxIndex++

		if spacerBoxIndex > maxSpacerBoxIndex {
			panic("Too many spacer boxes")
		}

		return &boxlayout.Box{Window: fmt.Sprintf("%s%d", statusSpacerPrefix, spacerBoxIndex), Weight: 1}
	}

	// Adds spacer boxes inbetween given boxes
	insertSpacerBoxes := func(boxes []*boxlayout.Box) []*boxlayout.Box {
		for i := len(boxes) - 1; i >= 1; i-- {
			// ignore existing spacer boxes
			if !strings.HasPrefix(boxes[i].Window, statusSpacerPrefix) {
				boxes = slices.Insert(boxes, i, spacerBox())
			}
		}
		return boxes
	}

	// First collect the real views that we want to show, we'll add spacers in
	// between at the end
	var result []*boxlayout.Box

	if !self.c.InDemo() {
		// app status appears very briefly in demos and dislodges the caption,
		// so better not to show it at all
		if appStatus != "" {
			result = append(result, &boxlayout.Box{Window: "appStatus", Size: runewidth.StringWidth(appStatus)})
		}
	}

	if self.c.UserConfig.Gui.ShowBottomLine {
		result = append(result, &boxlayout.Box{Window: "options", Weight: 1})
	}

	if (!self.c.InDemo() && self.c.UserConfig.Gui.ShowBottomLine) || self.modeHelper.IsAnyModeActive() {
		result = append(result,
			&boxlayout.Box{
				Window: "information",
				// unlike appStatus, informationStr has various colors so we need to decolorise before taking the length
				Size: runewidth.StringWidth(utils.Decolorise(informationStr)),
			})
	}

	if len(result) == 2 && result[0].Window == "appStatus" {
		// Only status and information are showing; need to insert a flexible
		// spacer after the 1-width spacer between the two, so that information
		// is right-aligned

		result = slices.Insert(result, 1, flexibleSpacerBox())
	} else if len(result) == 1 {
		if result[0].Window == "information" {
			// Only information is showing; need to add a flexible spacer so
			// that information is right-aligned
			result = slices.Insert(result, 0, flexibleSpacerBox())
		} else {
			// Only status is showing; need to make it flexible so that it
			// extends over the whole width
			result[0].Size = 0
			result[0].Weight = 1
		}
	}

	if len(result) > 0 {
		// If we have at least one view, insert 1-char wide spacer boxes between them.
		result = insertSpacerBoxes(result)
	}

	return result
}

func (self *WindowArrangementHelper) splitMainPanelSideBySide() bool {
	if !self.c.State().GetRepoState().GetSplitMainPanel() {
		return false
	}

	mainPanelSplitMode := self.c.UserConfig.Gui.MainPanelSplitMode
	width, height := self.c.GocuiGui().Size()

	switch mainPanelSplitMode {
	case "vertical":
		return false
	case "horizontal":
		return true
	default:
		if width < 200 && height > 30 { // 2 80 character width panels + 40 width for side panel
			return false
		} else {
			return true
		}
	}
}

func (self *WindowArrangementHelper) getExtrasWindowSize(screenHeight int) int {
	if !self.c.State().GetShowExtrasWindow() {
		return 0
	}

	var baseSize int
	if self.c.CurrentStaticContext().GetKey() == context.COMMAND_LOG_CONTEXT_KEY {
		baseSize = 1000 // my way of saying 'fill the available space'
	} else if screenHeight < 40 {
		baseSize = 1
	} else {
		baseSize = self.c.UserConfig.Gui.CommandLogSize
	}

	frameSize := 2
	return baseSize + frameSize
}

// The stash window by default only contains one line so that it's not hogging
// too much space, but if you access it it should take up some space. This is
// the default behaviour when accordion mode is NOT in effect. If it is in effect
// then when it's accessed it will have weight 2, not 1.
func (self *WindowArrangementHelper) getDefaultStashWindowBox() *boxlayout.Box {
	stashWindowAccessed := false
	self.c.Context().ForEach(func(context types.Context) {
		if context.GetWindowName() == "stash" {
			stashWindowAccessed = true
		}
	})

	box := &boxlayout.Box{Window: "stash"}
	// if the stash window is anywhere in our stack we should enlargen it
	if stashWindowAccessed {
		box.Weight = 1
	} else {
		box.Size = 3
	}

	return box
}

func (self *WindowArrangementHelper) sidePanelChildren(width int, height int) []*boxlayout.Box {
	currentWindow := self.c.CurrentSideContext().GetWindowName()

	screenMode := self.c.State().GetRepoState().GetScreenMode()
	if screenMode == types.SCREEN_FULL || screenMode == types.SCREEN_HALF {
		fullHeightBox := func(window string) *boxlayout.Box {
			if window == currentWindow {
				return &boxlayout.Box{
					Window: window,
					Weight: 1,
				}
			} else {
				return &boxlayout.Box{
					Window: window,
					Size:   0,
				}
			}
		}

		return []*boxlayout.Box{
			fullHeightBox("status"),
			fullHeightBox("files"),
			fullHeightBox("branches"),
			fullHeightBox("commits"),
			fullHeightBox("stash"),
		}
	} else if height >= 28 {
		accordionMode := self.c.UserConfig.Gui.ExpandFocusedSidePanel
		accordionBox := func(defaultBox *boxlayout.Box) *boxlayout.Box {
			if accordionMode && defaultBox.Window == currentWindow {
				return &boxlayout.Box{
					Window: defaultBox.Window,
					Weight: 2,
				}
			}

			return defaultBox
		}

		return []*boxlayout.Box{
			{
				Window: "status",
				Size:   3,
			},
			accordionBox(&boxlayout.Box{Window: "files", Weight: 1}),
			accordionBox(&boxlayout.Box{Window: "branches", Weight: 1}),
			accordionBox(&boxlayout.Box{Window: "commits", Weight: 1}),
			accordionBox(self.getDefaultStashWindowBox()),
		}
	} else {
		squashedHeight := 1
		if height >= 21 {
			squashedHeight = 3
		}

		squashedSidePanelBox := func(window string) *boxlayout.Box {
			if window == currentWindow {
				return &boxlayout.Box{
					Window: window,
					Weight: 1,
				}
			} else {
				return &boxlayout.Box{
					Window: window,
					Size:   squashedHeight,
				}
			}
		}

		return []*boxlayout.Box{
			squashedSidePanelBox("status"),
			squashedSidePanelBox("files"),
			squashedSidePanelBox("branches"),
			squashedSidePanelBox("commits"),
			squashedSidePanelBox("stash"),
		}
	}
}
