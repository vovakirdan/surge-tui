package screens

// OpenFileMsg просит приложение открыть файл в редакторе.
type OpenFileMsg struct {
	FilePath string
}

// OpenDirectoryMsg просит открыть заданную директорию в файловом менеджере.
type OpenDirectoryMsg struct {
	Path string
}
