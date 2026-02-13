// === Tab switching ===
document.querySelectorAll('.nav-item').forEach(function (btn) {
  btn.addEventListener('click', function () {
    var tab = btn.dataset.tab;
    document.querySelectorAll('.nav-item').forEach(function (b) { b.classList.remove('active'); });
    btn.classList.add('active');
    document.querySelectorAll('.tab-content').forEach(function (t) { t.classList.add('hidden'); });
    document.getElementById('tab-' + tab).classList.remove('hidden');
  });
});

// === Consultar ===
document.getElementById('loginForm').addEventListener('submit', async function (e) {
  e.preventDefault();
  var btn = document.getElementById('btnSubmit');
  var errorBox = document.getElementById('errorBox');
  var results = document.getElementById('results');

  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span><span class="btn-text">Consultando...</span>';
  errorBox.classList.add('hidden');
  results.style.display = 'none';

  try {
    var resp = await fetch('/api/consultar', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ci: document.getElementById('ci').value })
    });
    var data = await resp.json();

    if (data.error) {
      errorBox.textContent = data.error;
      errorBox.classList.remove('hidden');
      return;
    }

    document.getElementById('infoBar').innerHTML =
      infoCard('Estudiante', data.usuario) +
      infoCard('Sede', data.sede) +
      infoCard('Carrera', data.carrera) +
      infoCard('Total cursadas', data.total);

    var list = document.getElementById('materiasList');
    list.innerHTML = '';

    if (data.pendientes && data.pendientes.length > 0) {
      document.getElementById('resultsTitle').textContent =
        data.pendientes.length + ' materia(s) pendiente(s)';
      data.pendientes.forEach(function (m, i) {
        var docente = m.ApellidoPaterno + ' ' + m.ApellidoMaterno + ' ' + m.NombreDocente;
        var card = document.createElement('div');
        card.className = 'materia-card';
        card.style.animationDelay = (i * 0.06) + 's';
        card.innerHTML =
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
            mfield('Estado', '<span class="badge-pending">Pendiente</span>') +
          '</div>';
        list.appendChild(card);
      });
    } else {
      list.innerHTML = '<div class="empty">Sin materias pendientes</div>';
      document.getElementById('resultsTitle').textContent = 'Todo al dia';
    }
    results.style.display = 'block';
  } catch (err) {
    errorBox.textContent = 'Error de conexion: ' + err.message;
    errorBox.classList.remove('hidden');
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<span class="btn-text">Consultar</span><span class="btn-arrow">&rarr;</span>';
  }
});

// === Importar CSV ===
var uploadArea = document.getElementById('uploadArea');
var csvFile = document.getElementById('csvFile');
var fileName = document.getElementById('fileName');
var btnImport = document.getElementById('btnImport');

uploadArea.addEventListener('click', function () { csvFile.click(); });
uploadArea.addEventListener('dragover', function (e) { e.preventDefault(); uploadArea.classList.add('dragover'); });
uploadArea.addEventListener('dragleave', function () { uploadArea.classList.remove('dragover'); });
uploadArea.addEventListener('drop', function (e) {
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

document.getElementById('importForm').addEventListener('submit', async function (e) {
  e.preventDefault();
  var resultDiv = document.getElementById('importResult');

  btnImport.disabled = true;
  btnImport.innerHTML = '<span class="spinner"></span><span class="btn-text">Importando...</span>';
  resultDiv.classList.add('hidden');

  var formData = new FormData();
  formData.append('archivo', csvFile.files[0]);

  try {
    var resp = await fetch('/api/importar', { method: 'POST', body: formData });
    var data = await resp.json();

    if (data.error) {
      resultDiv.className = 'alert alert-error';
      resultDiv.textContent = data.error;
    } else {
      resultDiv.className = 'alert alert-success';
      var msg = 'Importados: ' + data.importados + ' estudiantes';
      if (data.errores > 0) msg += ' | Errores: ' + data.errores;
      msg += ' | Total en BD: ' + data.totalEnBD;
      resultDiv.textContent = msg;
    }
    resultDiv.classList.remove('hidden');
  } catch (err) {
    resultDiv.className = 'alert alert-error';
    resultDiv.textContent = 'Error: ' + err.message;
    resultDiv.classList.remove('hidden');
  } finally {
    btnImport.disabled = false;
    btnImport.innerHTML = '<span class="btn-text">Importar</span><span class="btn-arrow">&rarr;</span>';
  }
});

// === Helpers ===
function infoCard(label, value) {
  return '<div class="info-card"><div class="info-card-label">' + label + '</div><div class="info-card-value">' + (value || '-') + '</div></div>';
}
function mfield(label, value) {
  return '<div class="materia-field"><div class="materia-field-label">' + label + '</div><div class="materia-field-value">' + (value || '-') + '</div></div>';
}
