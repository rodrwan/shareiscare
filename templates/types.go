package templates

// FileInfo contiene información sobre un archivo para mostrar en el listado
type FileInfo struct {
	Name  string
	Path  string
	Size  string
	IsDir bool
	IsAdmin bool
}

// Breadcrumb estructura para representar un elemento del breadcrumb
type Breadcrumb struct {
	Name string
	Path string
}

// IndexData estructura para pasar datos a la plantilla de índice
type IndexData struct {
	Title       string
	Directory   string
	Files       []FileInfo
	Breadcrumbs []Breadcrumb
}

// UploadData estructura para pasar datos a la plantilla de subida
type UploadData struct {
	Title     string
	Directory string
	Success   bool
	Message   string
}

// LoginData estructura para pasar datos a la plantilla de login
type LoginData struct {
	Title        string
	Username     string
	ErrorMessage string
}

// LayoutData contiene los datos para el layout de la página
type LayoutData struct {
	Title      string
	IsLoggedIn bool
	Username   string
}
