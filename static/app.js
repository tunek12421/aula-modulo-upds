// === Numpad ===
var ciInput = document.getElementById('ci');
document.querySelectorAll('.numpad-key[data-key]').forEach(function (key) {
  key.addEventListener('click', function () {
    var val = key.getAttribute('data-key');
    if (val === 'del') {
      ciInput.value = ciInput.value.slice(0, -1);
    } else {
      if (ciInput.value.length < 10) ciInput.value += val;
    }
  });
});

// === Consultar ===
document.getElementById('loginForm').addEventListener('submit', async function (e) {
  e.preventDefault();
  var btn = document.getElementById('btnSubmit');
  var errorBox = document.getElementById('errorBox');
  var results = document.getElementById('results');

  if (!ciInput.value.trim()) return;

  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>';
  errorBox.classList.add('hidden');
  results.classList.remove('visible');
  document.querySelector('.panel').classList.remove('hidden-panel');
  document.querySelector('.hero').classList.remove('hidden-panel');

  try {
    var resp = await fetch('/api/consultar', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ci: ciInput.value })
    });
    var data = await resp.json();

    if (data.error) {
      errorBox.textContent = data.error;
      errorBox.classList.remove('hidden');
      return;
    }

    var list = document.getElementById('materiasList');
    list.innerHTML = '';

    if (data.pendientes && data.pendientes.length > 0) {
      document.getElementById('resultsTitle').innerHTML =
        '<span class="results-name">' + (data.usuario || '') + '</span>' +
        '<span class="results-count">' + data.pendientes.length + ' materia(s) actual(es)</span>';
      data.pendientes.forEach(function (m, i) {
        var docente = m.ApellidoPaterno + ' ' + m.ApellidoMaterno + ' ' + m.NombreDocente;
        var aula = '-';
        var horarioText = m.Horario || '';
        var parts = horarioText.split(':');
        if (parts.length >= 2) {
          aula = parts[0].trim();
          horarioText = parts.slice(1).join(':').trim();
        }
        var card = document.createElement('div');
        card.className = 'materia-card';
        card.style.animationDelay = (i * 0.06) + 's';
        card.innerHTML =
          '<div class="materia-nombre">' + m.Materia + '</div>' +
          '<div class="materia-hero">' +
            '<div class="materia-hero-item">' +
              '<div class="materia-hero-label">Aula</div>' +
              '<div class="materia-hero-value materia-aula">' + aula + '</div>' +
            '</div>' +
            '<div class="materia-hero-item">' +
              '<div class="materia-hero-label">Horario</div>' +
              '<div class="materia-hero-value">' + horarioText + '</div>' +
            '</div>' +
            '<div class="materia-hero-item">' +
              '<div class="materia-hero-label">Docente</div>' +
              '<div class="materia-hero-value">' + docente + '</div>' +
            '</div>' +
          '</div>' +
          '<div class="materia-meta">' +
            '<span>' + m.Turno + '</span>' +
            '<span>' + m.Semestre + '</span>' +
            '<span>' + m.Descripcion + '</span>' +
          '</div>';
        list.appendChild(card);
      });
    } else {
      list.innerHTML = '<div class="empty">Sin materias actuales</div>';
      document.getElementById('resultsTitle').innerHTML =
        '<span class="results-name">' + (data.usuario || '') + '</span>' +
        '<span class="results-count">Todo al dia</span>';
    }
    results.classList.add('visible');
    document.querySelector('.panel').classList.add('hidden-panel');
    document.querySelector('.hero').classList.add('hidden-panel');
  } catch (err) {
    errorBox.textContent = 'Error de conexion: ' + err.message;
    errorBox.classList.remove('hidden');
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<span class="btn-text">IR</span>';
  }
});

// === Volver ===
document.getElementById('btnBack').addEventListener('click', function () {
  document.getElementById('results').classList.remove('visible');
  document.querySelector('.panel').classList.remove('hidden-panel');
  document.querySelector('.hero').classList.remove('hidden-panel');
  ciInput.value = '';
});
