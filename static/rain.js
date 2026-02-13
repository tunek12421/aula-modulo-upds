// === Rain on Windshield - BigWings Heartfelt technique ===
// Adapted from Martijn Steinrucken (BigWings) 2017
// CC BY-NC-SA 3.0 - https://www.shadertoy.com/view/ltffzl
(function () {
  var canvas = document.getElementById('bgCanvas');
  var renderer = new THREE.WebGLRenderer({ canvas: canvas, antialias: false });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(window.innerWidth, window.innerHeight);

  var scene = new THREE.Scene();
  var camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);

  var videoEl = null, videoStream = null, webcamTex = null;

  var uniforms = {
    uTime: { value: 0 },
    uResolution: {
      value: new THREE.Vector2(
        window.innerWidth * renderer.getPixelRatio(),
        window.innerHeight * renderer.getPixelRatio()
      )
    },
    uWebcam: { value: new THREE.Texture() },
    uUseWebcam: { value: 0.0 }
  };

  var vertCode = `
    void main() {
      gl_Position = vec4(position.xy, 0.0, 1.0);
    }
  `;

  var fragCode = `
    precision highp float;
    uniform float uTime;
    uniform vec2 uResolution;
    uniform sampler2D uWebcam;
    uniform float uUseWebcam;

    #define S(a, b, t) smoothstep(a, b, t)

    // --- Noise functions (BigWings) ---
    vec3 N13(float p) {
      vec3 p3 = fract(vec3(p) * vec3(.1031,.11369,.13787));
      p3 += dot(p3, p3.yzx + 19.19);
      return fract(vec3((p3.x+p3.y)*p3.z, (p3.x+p3.z)*p3.y, (p3.y+p3.z)*p3.x));
    }

    float N(float t) {
      return fract(sin(t * 12345.564) * 7658.76);
    }

    float Saw(float b, float t) {
      return S(0., b, t) * S(1., b, t);
    }

    // --- Animated drop layer ---
    // Returns: x = drop mask, y = trail mask
    vec2 DropLayer2(vec2 uv, float t) {
      vec2 UV = uv;

      // Slow fall speed for windshield feel
      uv.y += t * 0.35;

      vec2 a = vec2(6., 1.);
      vec2 grid = a * 2.;
      vec2 id = floor(uv * grid);

      // Column shift for randomization
      float colShift = N(id.x);
      uv.y += colShift;

      id = floor(uv * grid);
      vec3 n = N13(id.x * 35.2 + id.y * 2376.1);
      vec2 st = fract(uv * grid) - vec2(.5, 0);

      // Drop X position with wiggle (surface tension)
      float x = n.x - .5;
      float y = UV.y * 20.;
      float wiggle = sin(y + sin(y));
      x += wiggle * (.5 - abs(x)) * (n.z - .5);
      x *= .7;

      // Drop timing - Saw wave for smooth appear/disappear
      float ti = fract(t + n.z);
      y = (Saw(.85, ti) - .5) * .9 + .5;

      vec2 p = vec2(x, y);
      float d = length((st - p) * a.yx);
      float mainDrop = S(.4, .0, d);

      // Trail behind the drop
      float r = sqrt(S(1., y, st.y));
      float cd = abs(st.x - x);
      float trail = S(.23 * r, .15 * r * r, cd);
      float trailFront = S(-.02, .02, st.y - y);
      trail *= trailFront * r * r;

      // Tiny residual droplets in the trail wake
      y = UV.y;
      float trail2 = S(.2 * r, .0, cd);
      float droplets = max(0., (sin(y * (1. - y) * 120.) - st.y)) * trail2 * trailFront * n.z;
      y = fract(y * 10.) + (st.y - .5);
      float dd = length(st - vec2(x, y));
      droplets = S(.3, 0., dd);
      float m = mainDrop + droplets * r * trailFront;

      return vec2(m, trail);
    }

    // --- Static condensation drops ---
    float StaticDrops(vec2 uv, float t) {
      uv *= 40.;
      vec2 id = floor(uv);
      uv = fract(uv) - .5;
      vec3 n = N13(id.x * 107.45 + id.y * 3543.654);
      vec2 p = (n.xy - .5) * .7;
      float d = length(uv - p);
      float fade = Saw(.025, fract(t + n.z));
      float c = S(.3, 0., d) * fract(n.z * 10.) * fade;
      return c;
    }

    // --- Combine all drop layers ---
    vec2 Drops(vec2 uv, float t, float l0, float l1, float l2) {
      float s = StaticDrops(uv, t) * l0;
      vec2 m1 = DropLayer2(uv, t) * l1;
      vec2 m2 = DropLayer2(uv * 1.85, t) * l2;

      float c = s + m1.x + m2.x;
      c = S(.3, 1., c);

      return vec2(c, max(m1.y * l0, m2.y * l1));
    }

    void main() {
      vec2 uv = (gl_FragCoord.xy - .5 * uResolution.xy) / uResolution.y;
      vec2 UV = gl_FragCoord.xy / uResolution.xy;

      float T = uTime;
      float t = T * 0.08;

      // Rain intensity
      float rainAmount = 0.7;

      float staticDrops = S(-.5, 1., rainAmount) * 2.;
      float layer1 = S(.25, .75, rainAmount);
      float layer2 = S(.0, .5, rainAmount);

      // Calculate drops
      vec2 c = Drops(uv, t, staticDrops, layer1, layer2);

      // Normal map via finite differences (key for realistic refraction)
      vec2 e = vec2(.001, 0.);
      float cx = Drops(uv + e, t, staticDrops, layer1, layer2).x;
      float cy = Drops(uv + e.yx, t, staticDrops, layer1, layer2).x;
      vec2 n = vec2(cx - c.x, cy - c.x);

      // Focus: trail areas get more blur, drops get sharp view
      float maxBlur = mix(3., 6., rainAmount);
      float minBlur = 2.;
      float focus = mix(maxBlur - c.y, minBlur, S(.1, .2, c.x));

      vec3 col;

      if (uUseWebcam > 0.5) {
        vec2 fuv = vec2(1.0 - UV.x, UV.y); // mirror for selfie

        // Fogged glass: variable blur based on focus
        float b = focus * 0.003;
        vec3 foggy = vec3(0.0);
        foggy += texture2D(uWebcam, fuv).rgb;
        foggy += texture2D(uWebcam, fuv + vec2(b, 0.0)).rgb;
        foggy += texture2D(uWebcam, fuv - vec2(b, 0.0)).rgb;
        foggy += texture2D(uWebcam, fuv + vec2(0.0, b)).rgb;
        foggy += texture2D(uWebcam, fuv - vec2(0.0, b)).rgb;
        foggy += texture2D(uWebcam, fuv + vec2(b, b) * 0.7).rgb;
        foggy += texture2D(uWebcam, fuv - vec2(b, b) * 0.7).rgb;
        foggy += texture2D(uWebcam, fuv + vec2(b, -b) * 0.7).rgb;
        foggy += texture2D(uWebcam, fuv - vec2(b, -b) * 0.7).rgb;
        foggy /= 9.0;
        foggy *= 0.35;
        foggy += vec3(0.01, 0.015, 0.025);

        // Clear through drops: refracted webcam (normal-based displacement)
        vec3 clear = texture2D(uWebcam, fuv + n * 0.5).rgb * 0.8;

        // Drops clear the fog
        col = mix(foggy, clear, c.x);

        // Subtle specular highlight on water
        col += vec3(0.04, 0.05, 0.08) * c.x;
      } else {
        // No webcam: dark moody background with visible rain
        vec3 bg = mix(vec3(0.015, 0.02, 0.045), vec3(0.008, 0.012, 0.025), UV.y);

        // Rain drops catch ambient light
        col = bg;
        col += vec3(0.06, 0.08, 0.15) * c.x;
        col += vec3(0.015, 0.02, 0.04) * c.y;

        // Specular highlight on drops
        col += vec3(0.08, 0.1, 0.18) * pow(c.x, 3.0);
      }

      gl_FragColor = vec4(col, 1.0);
    }
  `;

  var material = new THREE.ShaderMaterial({
    uniforms: uniforms,
    vertexShader: vertCode,
    fragmentShader: fragCode
  });

  scene.add(new THREE.Mesh(new THREE.PlaneGeometry(2, 2), material));

  var clock = new THREE.Clock();
  function animate() {
    requestAnimationFrame(animate);
    uniforms.uTime.value = clock.getElapsedTime();
    if (webcamTex) webcamTex.needsUpdate = true;
    renderer.render(scene, camera);
  }
  animate();

  // === Webcam toggle ===
  window.toggleCamera = function () {
    var btn = document.getElementById('camToggle');
    var label = btn.querySelector('span');
    if (uniforms.uUseWebcam.value > 0.5) {
      uniforms.uUseWebcam.value = 0.0;
      if (videoStream) {
        videoStream.getTracks().forEach(function (t) { t.stop(); });
        videoStream = null;
      }
      btn.classList.remove('active');
      label.textContent = 'Webcam';
    } else {
      if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
        alert('Tu navegador no soporta acceso a la camara');
        return;
      }
      label.textContent = 'Conectando...';
      navigator.mediaDevices.getUserMedia({ video: { facingMode: 'user' } })
        .then(function (stream) {
          videoStream = stream;
          videoEl = document.createElement('video');
          videoEl.srcObject = stream;
          videoEl.setAttribute('playsinline', '');
          videoEl.play();
          webcamTex = new THREE.VideoTexture(videoEl);
          webcamTex.minFilter = THREE.LinearFilter;
          webcamTex.magFilter = THREE.LinearFilter;
          uniforms.uWebcam.value = webcamTex;
          uniforms.uUseWebcam.value = 1.0;
          btn.classList.add('active');
          label.textContent = 'Webcam On';
        })
        .catch(function (err) {
          label.textContent = 'Webcam';
          alert('No se pudo acceder a la camara: ' + err.message);
        });
    }
  };

  window.addEventListener('resize', function () {
    renderer.setSize(window.innerWidth, window.innerHeight);
    uniforms.uResolution.value.set(
      window.innerWidth * renderer.getPixelRatio(),
      window.innerHeight * renderer.getPixelRatio()
    );
  });
})();
