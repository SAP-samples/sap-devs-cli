package theme

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	lipglossv1 "github.com/charmbracelet/lipgloss"
)

// SAP Fiori dark palette — derived from the Fiori Horizon Evening theme.
var (
	FioriBackground = lipgloss.Color("#1B2B3A")
	FioriBorder     = lipgloss.Color("#354A5F")
	FioriText       = lipgloss.Color("#EDEDED")
	FioriMuted      = lipgloss.Color("#8C9BAA")
	FioriGreen      = lipgloss.Color("#00D68F")
	FioriBlue       = lipgloss.Color("#4DB8FF")
	FioriRed        = lipgloss.Color("#FF5C5C")
	FioriOrange     = lipgloss.Color("#F58B00")
	FioriTeal       = lipgloss.Color("#0ABAAC")
	FioriCard       = lipgloss.Color("#243447")
)

// ThemeFiori returns a huh theme matching the SAP Fiori Horizon Evening palette.
func ThemeFiori(_ bool) *huh.Styles {
	t := huh.ThemeBase(true)

	t.Focused.Base = t.Focused.Base.BorderForeground(FioriBlue)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(FioriBlue).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(FioriBlue).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(FioriMuted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(FioriRed)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(FioriRed)
	t.Focused.Directory = t.Focused.Directory.Foreground(FioriBlue)
	t.Focused.File = t.Focused.File.Foreground(FioriText)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(FioriGreen)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(FioriGreen)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(FioriGreen)
	t.Focused.Option = t.Focused.Option.Foreground(FioriText)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(FioriGreen)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(FioriGreen)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(FioriGreen).SetString("✓ ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(FioriText)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(FioriMuted).SetString("• ")
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(FioriBackground).Background(FioriBlue).Bold(true)
	t.Focused.Next = t.Focused.FocusedButton
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(FioriText).Background(FioriCard)

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(FioriGreen)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(FioriMuted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(FioriBlue)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description

	return t
}

// List view styles using lipgloss v1 (matching the Fiori palette).

func SelectedStyle() lipglossv1.Style {
	return lipglossv1.NewStyle().Background(lipglossv1.Color("#354A5F")).Foreground(lipglossv1.Color("#EDEDED"))
}

func HeaderStyle() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#8C9BAA")).Bold(true)
}

func LayerBadgeOfficial() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#00D68F"))
}

func LayerBadgeCompany() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#F58B00"))
}

func LayerBadgeUser() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#4DB8FF"))
}

func LayerBadgeProject() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#0ABAAC"))
}

func OverrideSuffix() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#F58B00"))
}

func DiffAdded() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#00D68F"))
}

func DiffEdited() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#F58B00"))
}

func DiffDeleted() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#FF5C5C"))
}

func DiffMuted() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#8C9BAA"))
}

func SelectedCheckbox() lipglossv1.Style {
	return lipglossv1.NewStyle().Foreground(lipglossv1.Color("#4DB8FF"))
}
