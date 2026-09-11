package gui

// Undo and Redo expose the same edit history used by keyboard handling to
// menus and toolbars, without synthesizing keys or depending on held modifiers.
func (this *CodeEditor) Undo() { this.undo() }
func (this *CodeEditor) Redo() { this.redo() }

// MarkSaved establishes the buffer snapshot used for tab dirty indicators.
// Hosts call it only after loading or successfully writing the document.
func (this *CodeEditor) MarkSaved()     { this.savedText = this.Text() }
func (this *CodeEditor) Modified() bool { return this.Text() != this.savedText }
