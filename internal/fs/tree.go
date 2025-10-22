package fs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileNode узел дерева файлов
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size"`
	Children []*FileNode `json:"children,omitempty"`
	Parent   *FileNode   `json:"-"`
	Level    int         `json:"level"`
	Expanded bool        `json:"expanded"`
}

// FileTree дерево файлов
type FileTree struct {
	Root        *FileNode
	FlatList    []*FileNode // Плоский список для навигации
	Selected    int
	ShowHidden  bool
	FilterSurge bool // Показывать только .sg файлы
}

// NewFileTree создает новое дерево файлов
func NewFileTree(rootPath string) (*FileTree, error) {
	tree := &FileTree{
		Selected:    0,
		ShowHidden:  false,
		FilterSurge: false,
	}

	root, err := tree.buildNode(rootPath, nil, 0)
	if err != nil {
		return nil, err
	}

	tree.Root = root
	tree.Root.Expanded = true
	tree.rebuildFlatList()

	return tree, nil
}

// buildNode строит узел дерева рекурсивно
func (ft *FileTree) buildNode(path string, parent *FileNode, level int) (*FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	node := &FileNode{
		Name:     filepath.Base(path),
		Path:     path,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Parent:   parent,
		Level:    level,
		Expanded: false,
	}

	// Для директорий загружаем содержимое
	if node.IsDir {
		entries, err := os.ReadDir(path)
		if err != nil {
			return node, nil // Возвращаем узел без детей если нет доступа
		}

		for _, entry := range entries {
			// Пропускаем скрытые файлы если не включен показ
			if !ft.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			// Фильтр по .sg файлам
			if ft.FilterSurge && !entry.IsDir() {
				if !strings.HasSuffix(entry.Name(), ".sg") {
					continue
				}
			}

			childPath := filepath.Join(path, entry.Name())
			child, err := ft.buildNode(childPath, node, level+1)
			if err != nil {
				continue // Пропускаем проблемные файлы
			}

			node.Children = append(node.Children, child)
		}

		// Сортируем: сначала директории, потом файлы, по алфавиту
		sort.Slice(node.Children, func(i, j int) bool {
			a, b := node.Children[i], node.Children[j]
			if a.IsDir != b.IsDir {
				return a.IsDir // Директории идут первыми
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		})
	}

	return node, nil
}

// rebuildFlatList пересобирает плоский список для навигации
func (ft *FileTree) rebuildFlatList() {
	ft.FlatList = []*FileNode{}
	if ft.Root != nil {
		ft.addToFlatList(ft.Root)
	}
}

// addToFlatList добавляет узел и его видимых детей в плоский список
func (ft *FileTree) addToFlatList(node *FileNode) {
	ft.FlatList = append(ft.FlatList, node)

	if node.IsDir && node.Expanded {
		for _, child := range node.Children {
			ft.addToFlatList(child)
		}
	}
}

// ToggleExpanded переключает состояние разворота директории
func (ft *FileTree) ToggleExpanded(index int) {
	if index < 0 || index >= len(ft.FlatList) {
		return
	}

	node := ft.FlatList[index]
	if node.IsDir {
		node.Expanded = !node.Expanded
		ft.rebuildFlatList()

		// Корректируем выбранный элемент если список изменился
		if ft.Selected >= len(ft.FlatList) {
			ft.Selected = len(ft.FlatList) - 1
		}
	}
}

// SetSelected устанавливает выбранный элемент
func (ft *FileTree) SetSelected(index int) {
	if len(ft.FlatList) == 0 {
		ft.Selected = 0
		return
	}

	if index < 0 {
		ft.Selected = 0
	} else if index >= len(ft.FlatList) {
		ft.Selected = len(ft.FlatList) - 1
	} else {
		ft.Selected = index
	}
}

// GetSelected возвращает выбранный узел
func (ft *FileTree) GetSelected() *FileNode {
	if ft.Selected < 0 || ft.Selected >= len(ft.FlatList) {
		return nil
	}
	return ft.FlatList[ft.Selected]
}

// SetShowHidden устанавливает показ скрытых файлов
func (ft *FileTree) SetShowHidden(show bool) {
	if ft.ShowHidden != show {
		ft.ShowHidden = show
		ft.Refresh()
	}
}

// SetFilterSurge устанавливает фильтр по .sg файлам
func (ft *FileTree) SetFilterSurge(filter bool) {
	if ft.FilterSurge != filter {
		ft.FilterSurge = filter
		ft.Refresh()
	}
}

// Refresh обновляет дерево из файловой системы
func (ft *FileTree) Refresh() {
	if ft.Root == nil {
		return
	}

	// Сохраняем состояние разворота
	expandedPaths := ft.getExpandedPaths(ft.Root)

	// Пересобираем дерево
	newRoot, err := ft.buildNode(ft.Root.Path, nil, 0)
	if err != nil {
		return
	}

	// Восстанавливаем состояние разворота
	ft.restoreExpandedPaths(newRoot, expandedPaths)

	ft.Root = newRoot
	ft.rebuildFlatList()

	// Корректируем выбранный элемент
	if ft.Selected >= len(ft.FlatList) {
		ft.Selected = len(ft.FlatList) - 1
	}
}

// getExpandedPaths собирает пути развернутых директорий
func (ft *FileTree) getExpandedPaths(node *FileNode) map[string]bool {
	expanded := make(map[string]bool)
	ft.collectExpandedPaths(node, expanded)
	return expanded
}

// collectExpandedPaths рекурсивно собирает развернутые пути
func (ft *FileTree) collectExpandedPaths(node *FileNode, expanded map[string]bool) {
	if node.IsDir && node.Expanded {
		expanded[node.Path] = true
		for _, child := range node.Children {
			ft.collectExpandedPaths(child, expanded)
		}
	}
}

// restoreExpandedPaths восстанавливает состояние разворота
func (ft *FileTree) restoreExpandedPaths(node *FileNode, expanded map[string]bool) {
	if node.IsDir && expanded[node.Path] {
		node.Expanded = true
		for _, child := range node.Children {
			ft.restoreExpandedPaths(child, expanded)
		}
	}
}

// GetDisplayName возвращает имя для отображения с учетом уровня
func (node *FileNode) GetDisplayName() string {
	indent := strings.Repeat("  ", node.Level)

	if node.IsDir {
		if node.Expanded {
			return indent + "📂 " + node.Name
		} else {
			return indent + "📁 " + node.Name
		}
	}

	// Иконки для файлов по расширению
	ext := strings.ToLower(filepath.Ext(node.Name))
	switch ext {
	case ".sg":
		return indent + "📜 " + node.Name
	case ".md":
		return indent + "📝 " + node.Name
	case ".json", ".yaml", ".yml":
		return indent + "⚙️ " + node.Name
	default:
		return indent + "📄 " + node.Name
	}
}
