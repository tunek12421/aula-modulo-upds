package main

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

//go:embed static
var staticFS embed.FS

type LoginResponse struct {
	Status  int        `json:"status"`
	Message *string    `json:"message"`
	Data    []UserData `json:"data"`
}

type UserData struct {
	Type            int    `json:"Type"`
	TypeDescripcion string `json:"TypeDescripcion"`
	SedeNombreCorto string `json:"SedeNombreCorto"`
	SedeNombre      string `json:"SedeNombre"`
	ClienteToken    string `json:"ClienteToken"`
	ClienteNombre   string `json:"ClienteNombre"`
	ClienteDNI      string `json:"ClienteDNI"`
	ClienteCorreo   string `json:"ClienteCorreo"`
	ClienteGeneroId int    `json:"ClienteGeneroId"`
	IdRN            int    `json:"IdRN"`
}

type Materia struct {
	IdInscripcionCarrera int    `json:"IdInscripcionCarrera"`
	NotaLiteral          string `json:"NotaLiteral"`
	Materia              string `json:"Materia"`
	MateriaID            int    `json:"MateriaID"`
	RegistroMateriaID    int    `json:"RegistroMateriaID"`
	Sigla                string `json:"Sigla"`
	NroPeriodo           int    `json:"NroPeriodo"`
	IdGrupo              int    `json:"IdGrupo"`
	Grupo                string `json:"Grupo"`
	SistemaEstudio       string `json:"SistemaEstudio"`
	Turno                string `json:"Turno"`
	Descripcion          string `json:"Descripcion"`
	IdGestion            int    `json:"IdGestion"`
	NroOrden             int    `json:"NroOrden"`
	NroOfertaPeriodo     int    `json:"NroOfertaPeriodo"`
	IdDocente            int    `json:"IdDocente"`
	Horario              string `json:"Horario"`
	Semestre             string `json:"Semestre"`
	ApellidoPaterno      string `json:"ApellidoPaterno"`
	ApellidoMaterno      string `json:"ApellidoMaterno"`
	NombreDocente        string `json:"NombreDocente"`
	DocumentoIdentidad   string `json:"DocumentoIdentidad"`
	NumeroPeriodo        int    `json:"NumeroPeriodo"`
}

type Carrera struct {
	ID               int `json:"ID"`
	SistemaEnsenanza int `json:"SistemaEnsenanza"`
	PlanEstudio      struct {
		Nombre  string `json:"Nombre"`
		Sigla   string `json:"Sigla"`
		Carrera struct {
			Nombre string `json:"Nombre"`
		} `json:"Carrera"`
	} `json:"PlanEstudio"`
}

type ResultadoConsulta struct {
	Usuario    string    `json:"usuario"`
	Sede       string    `json:"sede"`
	Carrera    string    `json:"carrera"`
	Total      int       `json:"total"`
	Pendientes []Materia `json:"pendientes"`
	Error      string    `json:"error,omitempty"`
}

var secretKey []byte

const baseURL = "https://portal.upds.edu.bo"

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func doRequest(client *http.Client, method, url string, body io.Reader) (string, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("Accept", "application/json, text/html, */*")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

func login(client *http.Client, ci, pin string) (*UserData, error) {
	payload := fmt.Sprintf(`{"UserName":"%s","Password":"%s"}`, ci, pin)
	resp, err := doRequest(client, "POST",
		baseURL+"/gapi/request/service/?path=updsnet/access/cliente",
		strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error en autenticación: %w", err)
	}

	var loginResp LoginResponse
	if err := json.Unmarshal([]byte(resp), &loginResp); err != nil {
		return nil, fmt.Errorf("error parseando respuesta login: %w", err)
	}
	if loginResp.Status != 0 || len(loginResp.Data) == 0 {
		msg := "credenciales inválidas"
		if loginResp.Message != nil {
			msg = *loginResp.Message
		}
		return nil, fmt.Errorf("%s", msg)
	}

	userData := &loginResp.Data[0]
	sessionData, _ := json.Marshal(userData)
	_, err = doRequest(client, "POST",
		baseURL+"/updsnet/5.8/account/tlogin",
		strings.NewReader(string(sessionData)))
	if err != nil {
		return nil, fmt.Errorf("error creando sesión: %w", err)
	}

	return userData, nil
}

func obtenerInscripcion(client *http.Client) (inscripcion int, tipo int, carreraNombre string, err error) {
	resp, err := doRequest(client, "GET", baseURL+"/updsnet/5.8/home/registromateria", nil)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error obteniendo registro materia: %w", err)
	}

	re := regexp.MustCompile(`id="carreras-me"\s+value="([^"]+)"`)
	match := re.FindStringSubmatch(resp)
	if match == nil {
		return 0, 0, "", fmt.Errorf("no se encontró información de carreras")
	}

	decoded := html.UnescapeString(match[1])
	var carreras []Carrera
	if err := json.Unmarshal([]byte(decoded), &carreras); err != nil {
		return 0, 0, "", fmt.Errorf("error parseando carreras: %w", err)
	}
	if len(carreras) == 0 {
		return 0, 0, "", fmt.Errorf("no se encontraron carreras")
	}

	c := carreras[0]
	return c.ID, c.SistemaEnsenanza, c.PlanEstudio.Carrera.Nombre, nil
}

func obtenerHistorico(client *http.Client, inscripcion, tipo int) ([]Materia, error) {
	url := fmt.Sprintf("%s/updsnet/5.8/Home/ShowHistoricoRegistro?inscripcion=%d&tipo=%d",
		baseURL, inscripcion, tipo)
	resp, err := doRequest(client, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo histórico: %w", err)
	}

	re := regexp.MustCompile(`id="items-data"\s+value="([^"]+)"`)
	match := re.FindStringSubmatch(resp)
	if match == nil {
		return nil, fmt.Errorf("no se encontró datos del histórico")
	}

	decoded := html.UnescapeString(match[1])
	var materias []Materia
	if err := json.Unmarshal([]byte(decoded), &materias); err != nil {
		return nil, fmt.Errorf("error parseando materias: %w", err)
	}

	return materias, nil
}

func filtrarPendientes(materias []Materia) []Materia {
	var pendientes []Materia
	for _, m := range materias {
		if m.NotaLiteral == "Pendiente" {
			pendientes = append(pendientes, m)
		}
	}
	return pendientes
}

func consultar(ci, pin string) ResultadoConsulta {
	client := newClient()

	userData, err := login(client, ci, pin)
	if err != nil {
		return ResultadoConsulta{Error: err.Error()}
	}

	// Intentar cache de inscripción
	inscripcion, tipo, carrera, cached := obtenerCache(ci)
	if !cached {
		inscripcion, tipo, carrera, err = obtenerInscripcion(client)
		if err != nil {
			return ResultadoConsulta{Error: err.Error()}
		}
		guardarCache(ci, inscripcion, tipo, carrera)
	}

	materias, err := obtenerHistorico(client, inscripcion, tipo)
	if err != nil {
		return ResultadoConsulta{Error: err.Error()}
	}

	pendientes := filtrarPendientes(materias)

	return ResultadoConsulta{
		Usuario:    userData.ClienteNombre,
		Sede:       userData.SedeNombre,
		Carrera:    carrera,
		Total:      len(materias),
		Pendientes: pendientes,
	}
}

// --- HTTP Handlers ---

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleConsulta(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CI string `json:"ci"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	ci := strings.TrimSpace(req.CI)
	if ci == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ResultadoConsulta{Error: "CI es requerido"})
		return
	}

	// Buscar PIN en BD
	pinEnc, err := obtenerPinEncriptado(ci)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ResultadoConsulta{Error: "CI no registrado. Importa el CSV primero."})
		return
	}

	pin, err := decrypt(pinEnc, secretKey)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ResultadoConsulta{Error: "Error desencriptando PIN"})
		return
	}

	resultado := consultar(ci, pin)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultado)
}

func handleImportar(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "error parseando formulario", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("archivo")
	if err != nil {
		http.Error(w, "archivo CSV requerido", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "Error leyendo CSV: " + err.Error()})
		return
	}

	var importados, errores int
	var detalleErrores []string

	for i, record := range records {
		// Saltar header si existe
		if i == 0 && (strings.EqualFold(record[0], "ci") || strings.EqualFold(record[0], "carnet")) {
			continue
		}
		if len(record) < 2 {
			errores++
			detalleErrores = append(detalleErrores, fmt.Sprintf("Fila %d: faltan columnas", i+1))
			continue
		}

		ci := strings.TrimSpace(record[0])
		pin := strings.TrimSpace(record[1])
		if ci == "" || pin == "" {
			errores++
			detalleErrores = append(detalleErrores, fmt.Sprintf("Fila %d: CI o PIN vacío", i+1))
			continue
		}

		pinEnc, err := encrypt(pin, secretKey)
		if err != nil {
			errores++
			detalleErrores = append(detalleErrores, fmt.Sprintf("Fila %d: error encriptando", i+1))
			continue
		}

		if err := guardarEstudiante(ci, pinEnc); err != nil {
			errores++
			detalleErrores = append(detalleErrores, fmt.Sprintf("Fila %d: error guardando en BD", i+1))
			continue
		}

		importados++
	}

	total, _ := contarEstudiantes()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"importados":    importados,
		"errores":       errores,
		"detalleErrors": detalleErrores,
		"totalEnBD":     total,
	})
}

func main() {
	godotenv.Load()

	key := os.Getenv("SECRET_KEY")
	if len(key) != 32 {
		fmt.Println("Error: SECRET_KEY debe tener exactamente 32 caracteres en .env")
		os.Exit(1)
	}
	secretKey = []byte(key)

	if err := initDB("estudiantes.db"); err != nil {
		fmt.Println("Error inicializando BD:", err)
		os.Exit(1)
	}

	count, _ := contarEstudiantes()
	fmt.Printf("BD inicializada: %d estudiantes registrados\n", count)

	sub, _ := fs.Sub(staticFS, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/consultar", handleConsulta)
	http.HandleFunc("/api/importar", handleImportar)

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	fmt.Printf("Servidor iniciado en http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Println("Error:", err)
	}
}
