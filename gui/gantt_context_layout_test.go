package gui

import (
	"fmt"
	"testing"

	"github.com/uk0/silk/i18n"
	"github.com/uk0/silk/paint"
)

func TestGanttTaskDialogMeasuresTranslatedLabelsAndLargeFonts(t *testing.T) {
	mode, font, translator := CurrentThemeMode(), Theme().Font, i18n.Default
	t.Cleanup(func() { SetThemeMode(mode); Theme().Font = font; i18n.Default = translator })
	i18n.Default = i18n.NewTranslator()
	i18n.Default.AddMany("zh-CN", map[string]string{
		"Start (YYYY-MM-DD)": "开始日期 (YYYY-MM-DD)", "Finish (inclusive)": "完成日期（包含当天）",
		"Predecessor IDs": "前置任务编号", "Task": "任务名称", "Progress (%)": "完成进度 (%)",
	})
	i18n.Default.AddMany("fr-FR", map[string]string{
		"Start (YYYY-MM-DD)": "Date de début (YYYY-MM-DD)", "Finish (inclusive)": "Date de fin (jour inclus)",
		"Predecessor IDs": "Identifiants des prédécesseurs",
	})
	for _, theme := range []ThemeMode{ThemeLight, ThemeDark} {
		SetThemeMode(theme)
		for _, locale := range []string{"en-US", "zh-CN", "fr-FR"} {
			i18n.SetLocale(locale)
			for _, size := range []int{14, 20, 28} {
				t.Run(fmt.Sprintf("theme-%d/%s/font-%d", theme, locale, size), func(t *testing.T) {
					Theme().Font = paint.NewFont(font.Family(), size, false, false)
					c := ganttContextFixture(t)
					ganttContextPresent(t, func(d *Dialog) DialogResult {
						w, h := d.dialogSize()
						d.SetSize(w, h)
						d.Layout()
						form := ganttContextForm(t, d)
						form.Layout()
						previousBottom := 0.0
						for _, row := range form.rows {
							label := row.label.(*Label)
							ext := label.effectiveFont().TextExtents(label.Text())
							// Right alignment starts the ink at label.Width-ext.Width.
							if label.Width()+.01 < ext.Width || label.Height()+.01 < label.SizeHints().Height {
								t.Fatalf("label %q clipped: bounds=%gx%g ink=%gx%g", label.Text(), label.Width(), label.Height(), ext.Width, label.SizeHints().Height)
							}
							if label.Y() < previousBottom || label.X()+label.Width()+form.Spacing() > row.widget.X()+.01 {
								t.Fatal("task form rows/columns overlap")
							}
							if row.widget.Width() < 280 || row.widget.X()+row.widget.Width() > form.Width()+.01 {
								t.Fatal("measured labels squeezed or overflowed input fields")
							}
							previousBottom = label.Y() + label.Height()
						}
						if previousBottom > form.Height()+.01 || form.Y()+previousBottom > d.btnBox.Y()-16+.01 {
							t.Fatal("large-font task fields overlap footer or leave content bounds")
						}
						return DialogCancel
					})
					c.editGanttTask("b", "")
				})
			}
		}
	}
}
