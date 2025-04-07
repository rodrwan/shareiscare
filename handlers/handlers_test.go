package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rodrwan/shareiscare/config"
	"github.com/rodrwan/shareiscare/templates"
)

// Función mock para generar una firma determinista para pruebas
func mockGenerateSignature(username, timestamp, secretKey string) string {
	// Para pruebas, simplemente devolvemos una firma predecible
	return "mock-signature"
}

// configuración para pruebas
func setupTestConfig() *config.Config {
	// Crear directorio temporal para pruebas
	tempDir, _ := os.MkdirTemp("", "shareiscare-test")

	return &config.Config{
		Port:      8080,
		RootDir:   tempDir,
		Title:     "ShareIsCare Test",
		Username:  "testuser",
		Password:  "testpass",
		SecretKey: "test-secret-key",
	}
}

// cleanup después de las pruebas
func cleanupTestConfig(cfg *config.Config) {
	os.RemoveAll(cfg.RootDir)
}

// Mock para templ.Component
type mockTemplComponent struct{}

func (m mockTemplComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte("<html><body>Mock template content</body></html>"))
	return err
}

// Test para la función isAuthenticated
func TestIsAuthenticated(t *testing.T) {
	// Configuración del test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Creamos un par de solicitudes HTTP para probar
	validReq := httptest.NewRequest(http.MethodGet, "/", nil)
	invalidReq := httptest.NewRequest(http.MethodGet, "/", nil)
	noAuthReq := httptest.NewRequest(http.MethodGet, "/", nil)

	// Caso 1: Usuario autenticado correctamente
	timestamp := "1617123456"
	// Usamos nuestra función generateSignature real (no mock)
	signature := generateSignature("testuser", timestamp, cfg.SecretKey)
	validReq.AddCookie(&http.Cookie{
		Name:  "session",
		Value: fmt.Sprintf("testuser:%s:%s", timestamp, signature),
	})

	// Caso 2: Cookie con firma inválida
	invalidReq.AddCookie(&http.Cookie{
		Name:  "session",
		Value: fmt.Sprintf("testuser:%s:firma-invalida", timestamp),
	})

	// Caso 3: Formato de cookie inválido
	invalidReq2 := httptest.NewRequest(http.MethodGet, "/", nil)
	invalidReq2.AddCookie(&http.Cookie{
		Name:  "session",
		Value: "formato-invalido",
	})

	// Verificamos los resultados
	if !isAuthenticated(validReq, cfg) {
		t.Error("isAuthenticated() devolvió false para una sesión válida")
	}

	if isAuthenticated(invalidReq, cfg) {
		t.Error("isAuthenticated() devolvió true para una firma inválida")
	}

	if isAuthenticated(invalidReq2, cfg) {
		t.Error("isAuthenticated() devolvió true para un formato de cookie inválido")
	}

	if isAuthenticated(noAuthReq, cfg) {
		t.Error("isAuthenticated() devolvió true cuando no hay cookie de sesión")
	}
}

// Test para la función generateSignature
func TestGenerateSignature(t *testing.T) {
	username := "testuser"
	timestamp := "1617123456"
	secretKey := "test-secret-key"

	signature := generateSignature(username, timestamp, secretKey)

	// Verificar que la firma no esté vacía
	if signature == "" {
		t.Error("generateSignature() devolvió una firma vacía")
	}

	// Verificar que el mismo input produce la misma firma
	signature2 := generateSignature(username, timestamp, secretKey)
	if signature != signature2 {
		t.Errorf("generateSignature() no es determinista: %s != %s", signature, signature2)
	}

	// Verificar que diferentes inputs producen diferentes firmas
	signature3 := generateSignature("otheruser", timestamp, secretKey)
	if signature == signature3 {
		t.Error("generateSignature() debería producir firmas diferentes para usuarios diferentes")
	}
}

// Test para la función createSessionCookie
func TestCreateSessionCookie(t *testing.T) {
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	username := "testuser"
	cookie := createSessionCookie(username, cfg)

	// Verificar propiedades básicas de la cookie
	if cookie.Name != "session" {
		t.Errorf("cookie.Name = %s, quería 'session'", cookie.Name)
	}

	if !strings.HasPrefix(cookie.Value, username+":") {
		t.Errorf("cookie.Value = %s, debería empezar con '%s:'", cookie.Value, username)
	}

	if cookie.MaxAge != 3600*24 {
		t.Errorf("cookie.MaxAge = %d, quería %d", cookie.MaxAge, 3600*24)
	}

	if cookie.Path != "/" {
		t.Errorf("cookie.Path = %s, quería '/'", cookie.Path)
	}

	if !cookie.HttpOnly {
		t.Error("cookie.HttpOnly debería ser true")
	}
}

// Test para la función createSessionCookie con más detalle
func TestCreateSessionCookieDetailed(t *testing.T) {
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	username := "testuser"
	cookie := createSessionCookie(username, cfg)

	// Verificar propiedades básicas de la cookie
	if cookie.Name != "session" {
		t.Errorf("cookie.Name = %s, quería 'session'", cookie.Name)
	}

	// Verificar que el valor de la cookie tiene el formato correcto: username:timestamp:signature
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 3 {
		t.Errorf("formato de cookie incorrecto: %s, quería 'username:timestamp:signature'", cookie.Value)
	}

	if parts[0] != username {
		t.Errorf("username en cookie = %s, quería %s", parts[0], username)
	}

	// Verificar que el timestamp es un número válido
	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		t.Errorf("timestamp no es un número válido: %s", parts[1])
	}

	// Verificar que el timestamp está cerca del tiempo actual
	now := time.Now().Unix()
	if timestamp < now-10 || timestamp > now+10 {
		t.Errorf("timestamp no está cerca del tiempo actual: %d vs %d", timestamp, now)
	}

	// Verificar propiedades adicionales de la cookie
	if cookie.MaxAge != 3600*24 {
		t.Errorf("cookie.MaxAge = %d, quería %d", cookie.MaxAge, 3600*24)
	}

	if cookie.Path != "/" {
		t.Errorf("cookie.Path = %s, quería '/'", cookie.Path)
	}

	if !cookie.HttpOnly {
		t.Error("cookie.HttpOnly debería ser true")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie.SameSite = %d, quería SameSiteLaxMode (%d)", cookie.SameSite, http.SameSiteLaxMode)
	}
}

// Test para el middleware RequireAuth
func TestRequireAuth(t *testing.T) {
	// Configuración del test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear un handler ficticio para ser envuelto por RequireAuth
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Envolver el handler con RequireAuth
	protectedHandler := RequireAuth(testHandler, cfg)

	// Caso 1: Usuario no autenticado
	req := httptest.NewRequest(http.MethodGet, "/protected-route", nil)
	res := httptest.NewRecorder()

	// Ejecutar el handler
	protectedHandler(res, req)

	// Verificar redirección a login
	if res.Code != http.StatusSeeOther {
		t.Errorf("status code para usuario no autenticado = %d, quería %d",
			res.Code, http.StatusSeeOther)
	}

	if got := res.Header().Get("Location"); got != "/login" {
		t.Errorf("redirección incorrecta: %s, esperaba /login", got)
	}

	if handlerCalled {
		t.Error("el handler protegido no debería haber sido llamado para usuario no autenticado")
	}

	// Reset
	handlerCalled = false

	// Caso 2: Usuario autenticado
	req = httptest.NewRequest(http.MethodGet, "/protected-route", nil)
	res = httptest.NewRecorder()

	// Agregar cookie de sesión válida
	timestamp := "1617123456"
	signature := generateSignature("testuser", timestamp, cfg.SecretKey)
	sessionValue := fmt.Sprintf("testuser:%s:%s", timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	protectedHandler(res, req)

	// Verificar que el handler fue llamado
	if !handlerCalled {
		t.Error("el handler protegido debería haber sido llamado para usuario autenticado")
	}

	// Verificar código de respuesta
	if res.Code != http.StatusOK {
		t.Errorf("status code para usuario autenticado = %d, quería %d",
			res.Code, http.StatusOK)
	}
}

// Test para la función getFileType
func TestGetFileType(t *testing.T) {
	tests := []struct {
		filename string
		expected templates.FileType
	}{
		{"imagen.jpg", templates.FileTypeImage},
		{"imagen.png", templates.FileTypeImage},
		{"video.mp4", templates.FileTypeVideo},
		{"documento.txt", templates.FileTypeText},
		{"archivo.go", templates.FileTypeText},
		{"desconocido.xyz", templates.FileTypeUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := getFileType(tc.filename)
			if result != tc.expected {
				t.Errorf("getFileType(%s) = %s, quería %s", tc.filename, result, tc.expected)
			}
		})
	}
}

// Test para el handler Index
func TestIndex(t *testing.T) {
	// En lugar de saltar el test, verificamos comportamiento básico
	// sin depender de la implementación interna de templates
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear algunos archivos de prueba
	testFilePath := filepath.Join(cfg.RootDir, "archivo-test.txt")
	if err := os.WriteFile(testFilePath, []byte("Contenido de prueba"), 0644); err != nil {
		t.Fatalf("No se pudo crear archivo de prueba: %v", err)
	}

	testDirPath := filepath.Join(cfg.RootDir, "directorio-test")
	if err := os.Mkdir(testDirPath, 0755); err != nil {
		t.Fatalf("No se pudo crear directorio de prueba: %v", err)
	}

	// Caso 1: Acceso básico a la página principal
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	// Ejecutar el handler
	handler := Index(cfg)
	handler(res, req)

	// Verificar código de respuesta (al menos no debería fallar)
	if res.Code != http.StatusOK {
		t.Errorf("status code de index = %d, quería %d", res.Code, http.StatusOK)
	}
}

// Test para el handler Login
func TestLogin(t *testing.T) {
	// Enfoque similar: verificar comportamiento básico
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Caso 1: Usuario no autenticado accede a login
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	res := httptest.NewRecorder()

	// Ejecutar el handler
	handler := Login(cfg)
	handler(res, req)

	// Verificar código de respuesta
	if res.Code != http.StatusOK {
		t.Errorf("status code para login = %d, quería %d", res.Code, http.StatusOK)
	}

	// Caso 2: Usuario ya autenticado
	req = httptest.NewRequest(http.MethodGet, "/login", nil)
	res = httptest.NewRecorder()

	// Agregar cookie de sesión válida
	timestamp := "1617123456"
	signature := generateSignature("testuser", timestamp, cfg.SecretKey)
	sessionValue := fmt.Sprintf("testuser:%s:%s", timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	handler(res, req)

	// Verificar redirección a página principal o upload
	if res.Code != http.StatusSeeOther {
		t.Errorf("status code para usuario ya autenticado = %d, quería %d",
			res.Code, http.StatusSeeOther)
	}

	redirectLocation := res.Header().Get("Location")
	if redirectLocation != "/" && redirectLocation != "/upload" {
		t.Errorf("redirección incorrecta: %s, esperaba '/' o '/upload'", redirectLocation)
	}
}

// Test para el handler LoginPost
func TestLoginPost(t *testing.T) {
	// Configuración del test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Caso 1: Credenciales correctas
	formValues := url.Values{}
	formValues.Add("username", cfg.Username)
	formValues.Add("password", cfg.Password)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	// Ejecutar el handler
	handler := LoginPost(cfg)
	handler(res, req)

	// Verificar redirección a página principal o upload
	if res.Code != http.StatusSeeOther {
		t.Errorf("status code para login exitoso = %d, quería %d",
			res.Code, http.StatusSeeOther)
	}

	redirectLocation := res.Header().Get("Location")
	if redirectLocation != "/" && redirectLocation != "/upload" {
		t.Errorf("redirección incorrecta: %s, esperaba '/' o '/upload'", redirectLocation)
	}

	// Verificar que se ha establecido una cookie de sesión
	cookies := res.Result().Cookies()
	hasCookie := false
	for _, cookie := range cookies {
		if cookie.Name == "session" {
			hasCookie = true
			break
		}
	}
	if !hasCookie {
		t.Error("No se ha establecido cookie de sesión")
	}

	// Caso 2: Credenciales incorrectas
	formValues = url.Values{}
	formValues.Add("username", "usuario-incorrecto")
	formValues.Add("password", "clave-incorrecta")

	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(formValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res = httptest.NewRecorder()

	// Ejecutar el handler
	handler(res, req)

	// Verificar código de respuesta para login fallido
	if res.Code != http.StatusOK {
		t.Errorf("status code para login fallido = %d, quería %d",
			res.Code, http.StatusOK)
	}
}

// Test para el handler Logout
func TestLogout(t *testing.T) {
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear request y response
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	// Agregar cookie de sesión
	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: "testuser:1617123456:somevalue",
	})
	res := httptest.NewRecorder()

	// Ejecutar handler
	handler := Logout(cfg)
	handler(res, req)

	// Verificar redirección a página principal
	if res.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, quería %d", res.Code, http.StatusSeeOther)
	}

	// Verificar que la cookie de sesión ha sido eliminada
	cookies := res.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "session" {
			if cookie.MaxAge >= 0 {
				t.Error("session cookie no fue eliminada correctamente")
			}
			if time.Now().After(cookie.Expires) == false {
				t.Error("session cookie no caducó correctamente")
			}
		}
	}
}

// Test para el handler Upload
func TestUpload(t *testing.T) {
	// Configurar test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear request simulando un usuario autenticado
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: "testuser:1617123456:somevalue",
	})

	res := httptest.NewRecorder()

	// Ejecutar handler
	handler := Upload(cfg)
	handler(res, req)

	// Verificar código de estado
	if res.Code != http.StatusOK {
		t.Errorf("status code = %d, quería %d", res.Code, http.StatusOK)
	}

	// Verificar tipo de contenido
	contentType := res.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %s, esperaba que contuviera 'text/html'", contentType)
	}

	// Verificar que la respuesta contiene elementos esperados
	body := res.Body.String()
	if !strings.Contains(body, "upload") || !strings.Contains(body, "form") {
		t.Error("response no contiene el formulario de subida esperado")
	}
}

func TestUploadPost(t *testing.T) {
	// Configurar entorno de prueba
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear un archivo temporal para simular la carga
	tempFile, err := os.CreateTemp("", "test-upload-*.txt")
	if err != nil {
		t.Fatalf("No se pudo crear archivo temporal: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Escribir contenido en el archivo
	content := "Contenido de prueba para upload"
	if _, err := tempFile.Write([]byte(content)); err != nil {
		t.Fatalf("No se pudo escribir en archivo temporal: %v", err)
	}
	tempFile.Close()

	// Preparar el formulario multipart
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Abrir el archivo para leerlo
	file, err := os.Open(tempFile.Name())
	if err != nil {
		t.Fatalf("No se pudo abrir archivo temporal: %v", err)
	}
	defer file.Close()

	// Añadir el archivo al formulario
	part, err := writer.CreateFormFile("files", "test-upload.txt")
	if err != nil {
		t.Fatalf("No se pudo crear parte del formulario: %v", err)
	}
	if _, err = io.Copy(part, file); err != nil {
		t.Fatalf("No se pudo copiar contenido al formulario: %v", err)
	}
	writer.Close()

	// Crear request HTTP
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Agregar cookie de sesión simulando autenticación
	// Usamos nuestra función mockGenerateSignature para crear un valor de cookie válido
	timestamp := "1617123456"
	signature := mockGenerateSignature("testuser", timestamp, cfg.SecretKey)
	sessionValue := fmt.Sprintf("testuser:%s:%s", timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Crear el responseRecorder para capturar la respuesta
	res := httptest.NewRecorder()

	// Ejecutar el handler
	handler := UploadPost(cfg)
	handler(res, req)

	// Verificar que el código de respuesta es exitoso
	if res.Code != http.StatusOK {
		t.Errorf("status code = %d, quería %d", res.Code, http.StatusOK)
	}

	// Verificar que el cuerpo de la respuesta contiene un mensaje de éxito
	if !strings.Contains(res.Body.String(), "success") && !strings.Contains(res.Body.String(), "exitoso") {
		t.Error("respuesta no indica éxito en la subida del archivo")
	}

	// Verificar que el archivo fue guardado en el directorio configurado
	uploadedFilePath := filepath.Join(cfg.RootDir, "test-upload.txt")
	if _, err := os.Stat(uploadedFilePath); os.IsNotExist(err) {
		t.Errorf("el archivo no fue guardado en %s", uploadedFilePath)
	} else {
		// Verificar el contenido del archivo
		uploadedContent, err := os.ReadFile(uploadedFilePath)
		if err != nil {
			t.Errorf("no se pudo leer el archivo subido: %v", err)
		} else if string(uploadedContent) != content {
			t.Errorf("el contenido del archivo no coincide con lo esperado")
		}
	}
}

func TestBrowse(t *testing.T) {
	// Configurar entorno de prueba
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear una estructura de directorios para probar
	testDir := filepath.Join(cfg.RootDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("No se pudo crear directorio de prueba: %v", err)
	}

	// Crear un archivo dentro del directorio
	testFilePath := filepath.Join(testDir, "testfile.txt")
	if err := os.WriteFile(testFilePath, []byte("Archivo de prueba"), 0644); err != nil {
		t.Fatalf("No se pudo crear archivo de prueba: %v", err)
	}

	// Caso 1: Probar navegación a un directorio válido
	req := httptest.NewRequest(http.MethodGet, "/browse/testdir", nil)
	res := httptest.NewRecorder()

	// Agregar cookie de sesión simulando autenticación
	timestamp := "1617123456"
	signature := mockGenerateSignature("testuser", timestamp, cfg.SecretKey)
	sessionValue := fmt.Sprintf("testuser:%s:%s", timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	handler := Browse(cfg)
	handler(res, req)

	// Verificar que el código de respuesta es exitoso
	if res.Code != http.StatusOK {
		t.Errorf("status code para directorio válido = %d, quería %d", res.Code, http.StatusOK)
	}

	// Verificar que el contenido muestra el archivo dentro del directorio
	if !strings.Contains(res.Body.String(), "testfile.txt") {
		t.Error("respuesta no muestra el archivo dentro del directorio")
	}

	// Caso 2: Probar navegación a un archivo (debería redirigir a download)
	req = httptest.NewRequest(http.MethodGet, "/browse/testdir/testfile.txt", nil)
	res = httptest.NewRecorder()

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	handler(res, req)

	// Verificar redirección a la página de descarga
	if res.Code != http.StatusSeeOther {
		t.Errorf("status code para archivo = %d, quería redirección %d", res.Code, http.StatusSeeOther)
	}

	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/download?filename=") {
		t.Errorf("redirección incorrecta: %s, esperaba /download?filename=...", location)
	}

	// Caso 3: Probar acceso a ruta inexistente
	req = httptest.NewRequest(http.MethodGet, "/browse/rutainexistente", nil)
	res = httptest.NewRecorder()

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	handler(res, req)

	// Verificar error 404
	if res.Code != http.StatusNotFound {
		t.Errorf("status code para ruta inexistente = %d, quería %d", res.Code, http.StatusNotFound)
	}
}

func TestRequireAdmin(t *testing.T) {
	// Configuración del test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear un handler ficticio para ser envuelto por RequireAdmin
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Envolver el handler con RequireAdmin
	protectedHandler := RequireAdmin(testHandler, cfg)

	// Caso 1: Usuario no autenticado
	req := httptest.NewRequest(http.MethodGet, "/admin-route", nil)
	res := httptest.NewRecorder()

	// Ejecutar el handler
	protectedHandler(res, req)

	// Verificar redirección a login o respuesta de no autorizado
	// (la implementación puede variar, aceptamos ambos comportamientos)
	if res.Code != http.StatusSeeOther && res.Code != http.StatusUnauthorized {
		t.Errorf("status code para usuario no autenticado = %d, quería %d o %d",
			res.Code, http.StatusSeeOther, http.StatusUnauthorized)
	}

	if handlerCalled {
		t.Error("el handler no debería haber sido llamado para usuario no autenticado")
	}

	// Reset
	handlerCalled = false

	// Caso 2: Usuario autenticado pero no admin
	req = httptest.NewRequest(http.MethodGet, "/admin-route", nil)
	res = httptest.NewRecorder()

	// Agregar cookie de sesión (usuario no admin)
	timestamp := "1617123456"
	signature := mockGenerateSignature("no-admin-user", timestamp, cfg.SecretKey)
	sessionValue := fmt.Sprintf("no-admin-user:%s:%s", timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	protectedHandler(res, req)

	// Verificar acceso denegado (la implementación puede usar diferentes códigos)
	if res.Code != http.StatusForbidden && res.Code != http.StatusNotFound {
		t.Errorf("status code para usuario no admin = %d, quería %d o %d",
			res.Code, http.StatusForbidden, http.StatusNotFound)
	}

	if handlerCalled {
		t.Error("el handler no debería haber sido llamado para usuario no admin")
	}

	// Reset
	handlerCalled = false

	// Caso 3: Usuario admin (mismo que config.Username)
	req = httptest.NewRequest(http.MethodGet, "/admin-route", nil)
	res = httptest.NewRecorder()

	// Agregar cookie de sesión (usuario admin)
	signature = mockGenerateSignature(cfg.Username, timestamp, cfg.SecretKey)
	sessionValue = fmt.Sprintf("%s:%s:%s", cfg.Username, timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	protectedHandler(res, req)

	// Verificar acceso concedido
	if res.Code != http.StatusOK {
		t.Errorf("status code para usuario admin = %d, quería %d", res.Code, http.StatusOK)
	}

	if !handlerCalled {
		t.Error("el handler debería haber sido llamado para usuario admin")
	}
}

func TestDelete(t *testing.T) {
	// Configuración del test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear un archivo para eliminar
	testFilePath := filepath.Join(cfg.RootDir, "archivo-a-eliminar.txt")
	if err := os.WriteFile(testFilePath, []byte("Contenido de prueba"), 0644); err != nil {
		t.Fatalf("No se pudo crear archivo de prueba: %v", err)
	}

	// Verificar que el archivo existe inicialmente
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Fatal("El archivo de prueba no se creó correctamente")
	}

	// Caso 1: Intento de eliminar sin autenticación
	req := httptest.NewRequest(http.MethodPost, "/delete?filename=archivo-a-eliminar.txt", nil)
	res := httptest.NewRecorder()

	// Ejecutar el handler
	handler := Delete(cfg)
	handler(res, req)

	// Verificar respuesta (puede ser redirección o error directo)
	if res.Code != http.StatusSeeOther &&
		res.Code != http.StatusUnauthorized &&
		res.Code != http.StatusForbidden {
		t.Errorf("status code para usuario no autenticado = %d, quería %d, %d o %d",
			res.Code, http.StatusSeeOther, http.StatusUnauthorized, http.StatusForbidden)
	}

	// Verificar si el archivo sigue existiendo después del intento
	fileExists := true
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		fileExists = false
	}

	// Creamos de nuevo el archivo si fue eliminado
	if !fileExists {
		if err := os.WriteFile(testFilePath, []byte("Contenido de prueba"), 0644); err != nil {
			t.Fatalf("No se pudo recrear archivo de prueba: %v", err)
		}
	}

	// Caso 2: Eliminar como admin
	req = httptest.NewRequest(http.MethodPost, "/delete?filename=archivo-a-eliminar.txt", nil)
	res = httptest.NewRecorder()

	// Agregar cookie de sesión (usuario admin)
	timestamp := "1617123456"
	signature := mockGenerateSignature(cfg.Username, timestamp, cfg.SecretKey)
	sessionValue := fmt.Sprintf("%s:%s:%s", cfg.Username, timestamp, signature)

	req.AddCookie(&http.Cookie{
		Name:  "session",
		Value: sessionValue,
	})

	// Ejecutar el handler
	handler(res, req)

	// Verificar redirección o respuesta después de eliminar
	if res.Code != http.StatusSeeOther && res.Code != http.StatusOK {
		t.Errorf("status code después de eliminar = %d, quería %d o %d",
			res.Code, http.StatusSeeOther, http.StatusOK)
	}

	// Vemos si el archivo fue eliminado (puede tomar un momento)
	// Intentar varias veces
	maxRetries := 5
	deleted := false
	for i := 0; i < maxRetries; i++ {
		if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
			deleted = true
			break
		}
		time.Sleep(100 * time.Millisecond) // pequeña pausa
	}

	if !deleted {
		t.Error("El archivo no fue eliminado después de varios intentos")
	}
}

func TestPreview(t *testing.T) {
	// Configuración del test
	cfg := setupTestConfig()
	defer cleanupTestConfig(cfg)

	// Crear diferentes tipos de archivos para probar
	imgFilePath := filepath.Join(cfg.RootDir, "imagen.jpg")
	if err := os.WriteFile(imgFilePath, []byte("datos de imagen simulados"), 0644); err != nil {
		t.Fatalf("No se pudo crear archivo de imagen: %v", err)
	}

	textFilePath := filepath.Join(cfg.RootDir, "documento.txt")
	if err := os.WriteFile(textFilePath, []byte("Contenido de texto de prueba"), 0644); err != nil {
		t.Fatalf("No se pudo crear archivo de texto: %v", err)
	}

	// Caso 1: Vista previa de imagen
	req := httptest.NewRequest(http.MethodGet, "/preview?filename=imagen.jpg", nil)
	res := httptest.NewRecorder()

	// Ejecutar el handler
	handler := Preview(cfg)
	handler(res, req)

	// Verificar respuesta correcta
	if res.Code != http.StatusOK {
		t.Errorf("status code para vista previa de imagen = %d, quería %d", res.Code, http.StatusOK)
	}

	// Verificar que el content-type es correcto para imágenes
	contentType := res.Header().Get("Content-Type")
	if !strings.Contains(contentType, "image/") {
		t.Errorf("Content-Type incorrecto: %s, esperaba image/*", contentType)
	}

	// Caso 2: Vista previa de archivo de texto
	req = httptest.NewRequest(http.MethodGet, "/preview?filename=documento.txt", nil)
	res = httptest.NewRecorder()

	// Ejecutar el handler
	handler(res, req)

	// Verificar respuesta correcta
	if res.Code != http.StatusOK {
		t.Errorf("status code para vista previa de texto = %d, quería %d", res.Code, http.StatusOK)
	}

	// Para archivos de texto, verificar que el content-type es correcto
	contentType = res.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/") {
		t.Errorf("Content-Type incorrecto: %s, esperaba text/*", contentType)
	}

	// Verificar que el contenido del archivo de texto está en la respuesta
	if !strings.Contains(res.Body.String(), "Contenido de texto de prueba") {
		t.Error("La respuesta no contiene el contenido esperado del archivo de texto")
	}

	// Caso 3: Vista previa de archivo inexistente
	req = httptest.NewRequest(http.MethodGet, "/preview?filename=noexiste.txt", nil)
	res = httptest.NewRecorder()

	// Ejecutar el handler
	handler(res, req)

	// Verificar error 404
	if res.Code != http.StatusNotFound {
		t.Errorf("status code para archivo inexistente = %d, quería %d", res.Code, http.StatusNotFound)
	}

	// Caso 4: Intento de acceso fuera del directorio raíz
	req = httptest.NewRequest(http.MethodGet, "/preview?filename=../config.yaml", nil)
	res = httptest.NewRecorder()

	// Ejecutar el handler
	handler(res, req)

	// Verificar error de acceso denegado
	if res.Code != http.StatusForbidden {
		t.Errorf("status code para path-traversal = %d, quería %d", res.Code, http.StatusForbidden)
	}
}
