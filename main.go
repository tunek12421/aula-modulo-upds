package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

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
	return c.ID, c.SistemaEnsenanza, c.PlanEstudio.Carrera.Nombre + " " + c.PlanEstudio.Sigla, nil
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pageHTML)
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

const pageHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>UPDS - Materias Pendientes</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
  .container { max-width: 900px; margin: 0 auto; padding: 20px; }
  h1 { text-align: center; margin: 30px 0 10px; font-size: 1.8em; color: #38bdf8; }
  .subtitle { text-align: center; color: #64748b; margin-bottom: 30px; font-size: 0.9em; }

  .tabs { display: flex; justify-content: center; gap: 0; margin-bottom: 24px; }
  .tab {
    padding: 10px 28px; background: #1e293b; border: 1px solid #334155; color: #94a3b8;
    cursor: pointer; font-weight: 600; font-size: 0.9em; transition: all 0.2s;
  }
  .tab:first-child { border-radius: 8px 0 0 8px; }
  .tab:last-child { border-radius: 0 8px 8px 0; }
  .tab.active { background: #0369a1; color: white; border-color: #0369a1; }

  .card {
    background: #1e293b; border-radius: 12px; padding: 30px;
    max-width: 500px; margin: 0 auto 30px; border: 1px solid #334155;
  }
  .form-group { margin-bottom: 16px; }
  .form-group label { display: block; margin-bottom: 6px; color: #94a3b8; font-size: 0.85em; font-weight: 600; }
  .form-group input, .form-group select {
    width: 100%; padding: 10px 14px; background: #0f172a; border: 1px solid #334155;
    border-radius: 8px; color: #e2e8f0; font-size: 1em; outline: none; transition: border-color 0.2s;
  }
  .form-group input:focus { border-color: #38bdf8; }
  .btn {
    width: 100%; padding: 12px; background: #0369a1; color: white; border: none;
    border-radius: 8px; font-size: 1em; font-weight: 600; cursor: pointer; transition: background 0.2s;
  }
  .btn:hover { background: #0284c7; }
  .btn:disabled { background: #334155; cursor: not-allowed; }
  .btn-green { background: #15803d; }
  .btn-green:hover { background: #16a34a; }

  .spinner { display: inline-block; width: 16px; height: 16px; border: 2px solid #ffffff40;
    border-top-color: white; border-radius: 50%; animation: spin 0.6s linear infinite; vertical-align: middle; margin-right: 8px; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .info-bar {
    background: #1e293b; border-radius: 10px; padding: 16px 20px; margin-bottom: 20px;
    display: flex; gap: 24px; flex-wrap: wrap; border: 1px solid #334155;
  }
  .info-item { display: flex; flex-direction: column; }
  .info-label { font-size: 0.7em; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
  .info-value { font-size: 0.95em; color: #e2e8f0; font-weight: 600; }
  .results-title { font-size: 1.1em; color: #38bdf8; margin-bottom: 12px; }

  .materia-card {
    background: #1e293b; border-radius: 10px; padding: 18px 20px; margin-bottom: 12px;
    border-left: 4px solid #f59e0b; border-right: 1px solid #334155;
    border-top: 1px solid #334155; border-bottom: 1px solid #334155;
  }
  .materia-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; flex-wrap: wrap; gap: 8px; }
  .materia-nombre { font-size: 1.1em; font-weight: 700; color: #f8fafc; }
  .materia-sigla { font-size: 0.85em; color: #38bdf8; background: #0c4a6e; padding: 2px 10px; border-radius: 20px; }
  .materia-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 8px; }
  .materia-field-label { font-size: 0.7em; color: #64748b; text-transform: uppercase; }
  .materia-field-value { font-size: 0.88em; color: #cbd5e1; }
  .badge-pendiente {
    background: #92400e; color: #fbbf24; padding: 2px 10px; border-radius: 20px;
    font-size: 0.8em; font-weight: 700;
  }
  .empty { text-align: center; padding: 40px; color: #64748b; }
  .error-msg { background: #450a0a; border: 1px solid #7f1d1d; color: #fca5a5; padding: 14px 18px; border-radius: 8px; margin-top: 16px; }
  .success-msg { background: #052e16; border: 1px solid #166534; color: #86efac; padding: 14px 18px; border-radius: 8px; margin-top: 16px; }

  .upload-area {
    border: 2px dashed #334155; border-radius: 10px; padding: 30px; text-align: center;
    cursor: pointer; transition: border-color 0.2s; margin-bottom: 16px;
  }
  .upload-area:hover { border-color: #38bdf8; }
  .upload-area.dragover { border-color: #38bdf8; background: #0c4a6e20; }
  .upload-icon { font-size: 2em; margin-bottom: 8px; color: #64748b; }
  .upload-text { color: #94a3b8; font-size: 0.9em; }
  .upload-filename { color: #38bdf8; font-weight: 600; margin-top: 8px; }
  .csv-example { background: #0f172a; border-radius: 8px; padding: 12px 16px; font-family: monospace;
    font-size: 0.85em; color: #94a3b8; margin-top: 12px; text-align: left; }

  .hidden { display: none !important; }
  #results { display: none; }
</style>
</head>
<body>
<div class="container">
  <h1>UPDS Materias Pendientes</h1>
  <p class="subtitle">Consulta tus materias pendientes solo con tu carnet</p>

  <div class="tabs">
    <div class="tab active" onclick="switchTab('consultar')">Consultar</div>
    <div class="tab" onclick="switchTab('importar')">Importar CSV</div>
  </div>

  <!-- TAB: Consultar -->
  <div id="tab-consultar">
    <div class="card">
      <form id="loginForm">
        <div class="form-group">
          <label>Carnet de Identidad</label>
          <input type="text" id="ci" placeholder="Ej: 9491537" required>
        </div>
        <button type="submit" class="btn" id="btnSubmit">Consultar</button>
      </form>
      <div id="errorBox" class="error-msg hidden"></div>
    </div>

    <div id="results">
      <div class="info-bar" id="infoBar"></div>
      <div class="results-title" id="resultsTitle"></div>
      <div id="materiasList"></div>
    </div>
  </div>

  <!-- TAB: Importar CSV -->
  <div id="tab-importar" class="hidden">
    <div class="card">
      <form id="importForm">
        <div class="upload-area" id="uploadArea">
          <div class="upload-icon">&#128196;</div>
          <div class="upload-text">Arrastra un archivo CSV aqui o haz click para seleccionar</div>
          <div class="upload-filename" id="fileName"></div>
          <input type="file" id="csvFile" accept=".csv" style="display:none">
        </div>
        <div class="csv-example">
          <strong>Formato esperado:</strong><br>
          ci,pin<br>
          9491537,325658<br>
          1234567,111222
        </div>
        <br>
        <button type="submit" class="btn btn-green" id="btnImport" disabled>Importar</button>
      </form>
      <div id="importResult" class="hidden"></div>
    </div>
  </div>
</div>

<script>
// Tabs
function switchTab(tab) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.getElementById('tab-consultar').classList.add('hidden');
  document.getElementById('tab-importar').classList.add('hidden');
  document.getElementById('tab-' + tab).classList.remove('hidden');
  event.target.classList.add('active');
}

// Consultar
document.getElementById('loginForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const btn = document.getElementById('btnSubmit');
  const errorBox = document.getElementById('errorBox');
  const results = document.getElementById('results');

  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>Consultando...';
  errorBox.classList.add('hidden');
  results.style.display = 'none';

  try {
    const resp = await fetch('/api/consultar', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ci: document.getElementById('ci').value })
    });
    const data = await resp.json();

    if (data.error) {
      errorBox.textContent = data.error;
      errorBox.classList.remove('hidden');
      return;
    }

    document.getElementById('infoBar').innerHTML =
      field('Estudiante', data.usuario) +
      field('Sede', data.sede) +
      field('Carrera', data.carrera) +
      field('Total cursadas', data.total);

    const list = document.getElementById('materiasList');
    list.innerHTML = '';

    if (data.pendientes && data.pendientes.length > 0) {
      document.getElementById('resultsTitle').textContent =
        data.pendientes.length + ' materia(s) pendiente(s)';
      data.pendientes.forEach(m => {
        const docente = m.ApellidoPaterno + ' ' + m.ApellidoMaterno + ' ' + m.NombreDocente;
        list.innerHTML += '<div class="materia-card">' +
          '<div class="materia-header">' +
            '<span class="materia-nombre">' + m.Materia + '</span>' +
            '<span class="materia-sigla">' + m.Sigla + '</span>' +
          '</div>' +
          '<div class="materia-grid">' +
            mfield('Periodo', m.Descripcion) +
            mfield('Grupo', m.Grupo) +
            mfield('Horario', m.Horario) +
            mfield('Turno', m.Turno) +
            mfield('Semestre', m.Semestre) +
            mfield('Docente', docente) +
            mfield('Sistema', m.SistemaEstudio) +
            mfield('Estado', '<span class="badge-pendiente">Pendiente</span>') +
          '</div>' +
        '</div>';
      });
    } else {
      list.innerHTML = '<div class="empty">No hay materias pendientes</div>';
      document.getElementById('resultsTitle').textContent = 'Sin pendientes';
    }
    results.style.display = 'block';
  } catch (err) {
    errorBox.textContent = 'Error de conexion: ' + err.message;
    errorBox.classList.remove('hidden');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Consultar';
  }
});

// Importar CSV
const uploadArea = document.getElementById('uploadArea');
const csvFile = document.getElementById('csvFile');
const fileName = document.getElementById('fileName');
const btnImport = document.getElementById('btnImport');

uploadArea.addEventListener('click', () => csvFile.click());
uploadArea.addEventListener('dragover', (e) => { e.preventDefault(); uploadArea.classList.add('dragover'); });
uploadArea.addEventListener('dragleave', () => uploadArea.classList.remove('dragover'));
uploadArea.addEventListener('drop', (e) => {
  e.preventDefault();
  uploadArea.classList.remove('dragover');
  if (e.dataTransfer.files.length) {
    csvFile.files = e.dataTransfer.files;
    onFileSelected();
  }
});
csvFile.addEventListener('change', onFileSelected);

function onFileSelected() {
  if (csvFile.files.length) {
    fileName.textContent = csvFile.files[0].name;
    btnImport.disabled = false;
  }
}

document.getElementById('importForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const resultDiv = document.getElementById('importResult');

  btnImport.disabled = true;
  btnImport.innerHTML = '<span class="spinner"></span>Importando...';
  resultDiv.classList.add('hidden');

  const formData = new FormData();
  formData.append('archivo', csvFile.files[0]);

  try {
    const resp = await fetch('/api/importar', { method: 'POST', body: formData });
    const data = await resp.json();

    if (data.error) {
      resultDiv.className = 'error-msg';
      resultDiv.textContent = data.error;
    } else {
      resultDiv.className = 'success-msg';
      let msg = 'Importados: ' + data.importados + ' estudiantes';
      if (data.errores > 0) msg += ' | Errores: ' + data.errores;
      msg += ' | Total en BD: ' + data.totalEnBD;
      resultDiv.textContent = msg;
    }
    resultDiv.classList.remove('hidden');
  } catch (err) {
    resultDiv.className = 'error-msg';
    resultDiv.textContent = 'Error: ' + err.message;
    resultDiv.classList.remove('hidden');
  } finally {
    btnImport.disabled = false;
    btnImport.textContent = 'Importar';
  }
});

function field(label, value) {
  return '<div class="info-item"><span class="info-label">' + label + '</span><span class="info-value">' + (value||'-') + '</span></div>';
}
function mfield(label, value) {
  return '<div class="materia-field"><div class="materia-field-label">' + label + '</div><div class="materia-field-value">' + (value||'-') + '</div></div>';
}
</script>
</body>
</html>`
