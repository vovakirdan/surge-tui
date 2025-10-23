package screens

// OpenLocationMsg requests the project workspace to open a file and position the cursor.
type OpenLocationMsg struct {
	FilePath string
	Line     int
	Column   int
}

// OpenFixModeMsg просит приложение открыть экран фиксов, опционально сфокусировав фикс.
type OpenFixModeMsg struct {
	FilePath string
	FixID    string
}
