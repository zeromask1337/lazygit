package presentation

import (
	"strings"

	"github.com/gookit/color"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/patch"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/icons"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/theme"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

const (
	EXPANDED_ARROW  = "▼"
	COLLAPSED_ARROW = "▶"
)

// keeping these here as individual constants in case later on people want the old tree shape
const (
	INNER_ITEM = "  "
	LAST_ITEM  = "  "
	NESTED     = "  "
	NOTHING    = "  "
)

func RenderFileTree(
	tree filetree.IFileTree,
	diffName string,
	submoduleConfigs []*models.SubmoduleConfig,
) []string {
	collapsedPaths := tree.CollapsedPaths()
	return renderAux(tree.GetRoot().Raw(), collapsedPaths, "", -1, func(node *filetree.Node[models.File], depth int) string {
		fileNode := filetree.NewFileNode(node)

		return getFileLine(collapsedPaths, fileNode.GetHasUnstagedChanges(), fileNode.GetHasStagedChanges(), fileNameAtDepth(node, depth), diffName, submoduleConfigs, node)
	})
}

func RenderCommitFileTree(
	tree *filetree.CommitFileTreeViewModel,
	diffName string,
	patchBuilder *patch.PatchBuilder,
) []string {
	collapsedPaths := tree.CollapsedPaths()
	return renderAux(tree.GetRoot().Raw(), collapsedPaths, "", -1, func(node *filetree.Node[models.CommitFile], depth int) string {
		// This is a little convoluted because we're dealing with either a leaf or a non-leaf.
		// But this code actually applies to both. If it's a leaf, the status will just
		// be whatever status it is, but if it's a non-leaf it will determine its status
		// based on the leaves of that subtree
		var status patch.PatchStatus
		if node.EveryFile(func(file *models.CommitFile) bool {
			return patchBuilder.GetFileStatus(file.Name, tree.GetRef().RefName()) == patch.WHOLE
		}) {
			status = patch.WHOLE
		} else if node.EveryFile(func(file *models.CommitFile) bool {
			return patchBuilder.GetFileStatus(file.Name, tree.GetRef().RefName()) == patch.UNSELECTED
		}) {
			status = patch.UNSELECTED
		} else {
			status = patch.PART
		}

		return getCommitFileLine(collapsedPaths, commitFileNameAtDepth(node, depth), diffName, node, status)
	})
}

func renderAux[T any](
	node *filetree.Node[T],
	collapsedPaths *filetree.CollapsedPaths,
	prefix string,
	depth int,
	renderLine func(*filetree.Node[T], int) string,
) []string {
	if node == nil {
		return []string{}
	}

	isRoot := depth == -1

	if node.IsFile() {
		if isRoot {
			return []string{}
		}
		return []string{prefix + renderLine(node, depth)}
	}

	arr := []string{}
	if !isRoot {
		arr = append(arr, prefix+renderLine(node, depth))
	}

	if collapsedPaths.IsCollapsed(node.GetPath()) {
		return arr
	}

	newPrefix := prefix
	if strings.HasSuffix(prefix, LAST_ITEM) {
		newPrefix = strings.TrimSuffix(prefix, LAST_ITEM) + NOTHING
	} else if strings.HasSuffix(prefix, INNER_ITEM) {
		newPrefix = strings.TrimSuffix(prefix, INNER_ITEM) + NESTED
	}

	for i, child := range node.Children {
		isLast := i == len(node.Children)-1

		var childPrefix string
		if isRoot {
			childPrefix = newPrefix
		} else if isLast {
			childPrefix = newPrefix + LAST_ITEM
		} else {
			childPrefix = newPrefix + INNER_ITEM
		}

		arr = append(arr, renderAux(child, collapsedPaths, childPrefix, depth+1+node.CompressionLevel, renderLine)...)
	}

	return arr
}

func getFileLine(collapsedPaths *filetree.CollapsedPaths, hasUnstagedChanges bool, hasStagedChanges bool, name string, diffName string, submoduleConfigs []*models.SubmoduleConfig, node *filetree.Node[models.File]) string {
	output := ""

	restColor := style.FgDefault

	file := node.File

	if file == nil {
		arrow := EXPANDED_ARROW
		if collapsedPaths.IsCollapsed(node.GetPath()) {
			arrow = COLLAPSED_ARROW
		}

		var arrowStyle style.TextStyle
		if hasStagedChanges && !hasUnstagedChanges {
			arrowStyle = style.FgGreen
		} else if hasStagedChanges && hasUnstagedChanges {
			arrowStyle = style.FgYellow
		} else {
			arrowStyle = style.FgRed
		}
		output += arrowStyle.Sprint(arrow) + " "
	} else {
		// this is just making things look nice when the background attribute is 'reverse'
		firstChar := file.ShortStatus[0:1]
		firstCharCl := style.FgGreen
		if firstChar == "?" {
			firstCharCl = theme.UnstagedChangesColor
		} else if firstChar == " " {
			firstCharCl = restColor
		}

		secondChar := file.ShortStatus[1:2]
		secondCharCl := theme.UnstagedChangesColor
		if secondChar == " " {
			secondCharCl = restColor
		}

		output = firstCharCl.Sprint(firstChar)
		output += secondCharCl.Sprint(secondChar)
		output += restColor.Sprint(" ")
	}

	isSubmodule := file != nil && file.IsSubmodule(submoduleConfigs)
	isLinkedWorktree := file != nil && file.IsWorktree
	isDirectory := file == nil

	if icons.IsIconEnabled() {
		icon := icons.IconForFile(name, isSubmodule, isLinkedWorktree, isDirectory)
		paint := color.C256(icon.Color, false)
		output += paint.Sprint(icon.Icon) + " "
	}

	output += restColor.Sprint(utils.EscapeSpecialChars(name))

	if isSubmodule {
		output += theme.DefaultTextColor.Sprint(" (submodule)")
	}

	return output
}

func getCommitFileLine(collapsedPaths *filetree.CollapsedPaths, name string, diffName string, node *filetree.Node[models.CommitFile], status patch.PatchStatus) string {
	commitFile := node.File
	output := ""

	if commitFile == nil {
		arrow := EXPANDED_ARROW
		if collapsedPaths.IsCollapsed(node.GetPath()) {
			arrow = COLLAPSED_ARROW
		}

		var arrowColour style.TextStyle
		if diffName == name {
			arrowColour = theme.DiffTerminalColor
		} else {
			switch status {
			case patch.WHOLE:
				arrowColour = style.FgGreen
			case patch.PART:
				arrowColour = style.FgYellow
			case patch.UNSELECTED:
				arrowColour = theme.DefaultTextColor
			}
		}

		output += arrowColour.Sprint(arrow) + " "
	} else {
		output += getColorForChangeStatus(commitFile.ChangeStatus).Sprint(commitFile.ChangeStatus) + " "
	}

	name = utils.EscapeSpecialChars(name)
	isSubmodule := false
	isLinkedWorktree := false
	isDirectory := commitFile == nil

	if icons.IsIconEnabled() {
		icon := icons.IconForFile(name, isSubmodule, isLinkedWorktree, isDirectory)
		paint := color.C256(icon.Color, false)
		output += paint.Sprint(icon.Icon) + " "
	}

	output += style.FgDefault.Sprint(name)
	return output
}

func getColorForChangeStatus(changeStatus string) style.TextStyle {
	switch changeStatus {
	case "A":
		return style.FgGreen
	case "M", "R":
		return style.FgYellow
	case "D":
		return theme.UnstagedChangesColor
	case "C":
		return style.FgCyan
	case "T":
		return style.FgMagenta
	default:
		return theme.DefaultTextColor
	}
}

func fileNameAtDepth(node *filetree.Node[models.File], depth int) string {
	splitName := split(node.Path)
	name := join(splitName[depth:])

	if node.File != nil && node.File.IsRename() {
		splitPrevName := split(node.File.PreviousName)

		prevName := node.File.PreviousName
		// if the file has just been renamed inside the same directory, we can shave off
		// the prefix for the previous path too. Otherwise we'll keep it unchanged
		sameParentDir := len(splitName) == len(splitPrevName) && join(splitName[0:depth]) == join(splitPrevName[0:depth])
		if sameParentDir {
			prevName = join(splitPrevName[depth:])
		}

		return prevName + " → " + name
	}

	return name
}

func commitFileNameAtDepth(node *filetree.Node[models.CommitFile], depth int) string {
	splitName := split(node.Path)
	name := join(splitName[depth:])

	return name
}

func split(str string) []string {
	return strings.Split(str, "/")
}

func join(strs []string) string {
	return strings.Join(strs, "/")
}
