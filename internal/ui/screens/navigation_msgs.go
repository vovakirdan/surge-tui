package screens

// OpenLocationMsg requests the project workspace to open a file and position the cursor.
type OpenLocationMsg struct {
	FilePath string
	Line     int
	Column   int
}
