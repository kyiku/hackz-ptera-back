// API Configuration
const API_BASE = 'http://localhost:8080';

// State
let currentStage = 'start';
let sessionActive = false;
let dinoScore = 0;
let captchaAttempts = 3;

// DOM Elements
const stages = {
    start: document.getElementById('stage-start'),
    waiting: document.getElementById('stage-waiting'),
    dino: document.getElementById('stage-dino'),
    captcha: document.getElementById('stage-captcha'),
    register: document.getElementById('stage-register'),
    failure: document.getElementById('stage-failure')
};

// Utility Functions
function log(type, message) {
    const logEl = document.getElementById('api-log');
    const timestamp = new Date().toLocaleTimeString();
    const entry = document.createElement('div');
    entry.className = `log-entry ${type}`;
    entry.innerHTML = `<span class="log-timestamp">[${timestamp}]</span> ${message}`;
    logEl.appendChild(entry);
    logEl.scrollTop = logEl.scrollHeight;
}

function showStage(stageName) {
    Object.values(stages).forEach(s => s.classList.remove('active'));
    if (stages[stageName]) {
        stages[stageName].classList.add('active');
        currentStage = stageName;
    }
}

function updateStatusBar(data) {
    if (data.user_id || sessionActive) {
        document.getElementById('session-status').textContent = 'Session: Active';
    }
    if (data.status) {
        document.getElementById('user-status').textContent = `Status: ${data.status}`;
    }
    if (data.position !== undefined) {
        document.getElementById('queue-position').textContent = `Queue: ${data.position}/${data.total || '?'}`;
        document.getElementById('queue-number').textContent = data.position;
    }
}

// API Functions
async function apiRequest(method, endpoint, body = null) {
    const url = `${API_BASE}${endpoint}`;
    log('request', `${method} ${endpoint}`);

    try {
        const options = {
            method,
            headers: {
                'Content-Type': 'application/json',
            },
            credentials: 'include', // Important for cookies
        };

        if (body) {
            options.body = JSON.stringify(body);
            log('request', `Body: ${JSON.stringify(body)}`);
        }

        const response = await fetch(url, options);
        const data = await response.json();

        log('response', `Status: ${response.status} - ${JSON.stringify(data)}`);
        return { ok: response.ok, status: response.status, data };
    } catch (error) {
        log('error', `Error: ${error.message}`);
        return { ok: false, error: error.message };
    }
}

// Stage Handlers
async function startSession() {
    const result = await apiRequest('POST', '/api/session');

    if (result.ok && !result.data.error) {
        sessionActive = true;
        updateStatusBar(result.data);
        showStage('waiting');

        // Start polling queue status
        startQueuePolling();
    } else {
        log('error', 'Failed to start session');
    }
}

let queuePollInterval = null;

function startQueuePolling() {
    if (queuePollInterval) clearInterval(queuePollInterval);

    queuePollInterval = setInterval(async () => {
        if (currentStage !== 'waiting') {
            clearInterval(queuePollInterval);
            return;
        }

        const result = await apiRequest('GET', '/api/queue/status');
        if (result.ok) {
            updateStatusBar(result.data);

            // Auto-advance based on status
            if (result.data.status === 'stage1_dino') {
                showStage('dino');
                startDinoGame();
            }
        }
    }, 3000);
}

// Dino Game
let dinoInterval = null;

function startDinoGame() {
    dinoScore = 0;
    updateDinoScore();

    // Simple score counter
    dinoInterval = setInterval(() => {
        dinoScore += 10;
        updateDinoScore();
    }, 100);

    // Jump handler
    document.addEventListener('keydown', handleDinoJump);
}

function handleDinoJump(e) {
    if (e.code === 'Space' && currentStage === 'dino') {
        e.preventDefault();
        const dino = document.getElementById('dino');
        if (!dino.classList.contains('jump')) {
            dino.classList.add('jump');
            setTimeout(() => dino.classList.remove('jump'), 500);
        }
    }
}

function updateDinoScore() {
    document.getElementById('dino-score').textContent = dinoScore;
}

function stopDinoGame() {
    if (dinoInterval) {
        clearInterval(dinoInterval);
        dinoInterval = null;
    }
    document.removeEventListener('keydown', handleDinoJump);
}

async function sendDinoResult(result) {
    stopDinoGame();

    const response = await apiRequest('POST', '/api/dino/result', {
        result: result,
        score: dinoScore
    });

    if (response.ok) {
        if (response.data.next_stage === 'stage2_captcha') {
            updateStatusBar({ status: 'stage2_captcha' });
            showStage('captcha');
        } else if (response.data.error) {
            showFailure(response.data.message);
        }
    }
}

// CAPTCHA
async function simulateCaptcha(success) {
    if (success) {
        // Simulate successful click
        updateStatusBar({ status: 'registering' });
        showStage('register');
    } else {
        captchaAttempts--;
        document.getElementById('captcha-attempts').textContent = captchaAttempts;

        if (captchaAttempts <= 0) {
            showFailure('3回失敗しました。待機列の最後尾からやり直しです。');
        }
    }
}

// Registration
async function submitRegistration(e) {
    e.preventDefault();

    const formData = {
        username: document.getElementById('username').value,
        email: document.getElementById('email').value,
        password: document.getElementById('password').value,
        token: 'mock-token'
    };

    const result = await apiRequest('POST', '/api/register/submit', formData);

    const resultBox = document.getElementById('register-result');

    if (result.data && result.data.error) {
        resultBox.className = 'result-box error';
        resultBox.textContent = result.data.message;

        if (result.data.redirect_delay) {
            showFailure(result.data.message);
        }
    } else {
        resultBox.className = 'result-box success';
        resultBox.textContent = 'Registration successful!';
    }
}

// Failure handling
function showFailure(message) {
    document.getElementById('failure-message').textContent = message;
    showStage('failure');

    let countdown = 3;
    const countdownEl = document.getElementById('redirect-countdown');

    const countdownInterval = setInterval(() => {
        countdown--;
        countdownEl.textContent = countdown;

        if (countdown <= 0) {
            clearInterval(countdownInterval);
            resetToQueue();
        }
    }, 1000);
}

function resetToQueue() {
    captchaAttempts = 3;
    document.getElementById('captcha-attempts').textContent = captchaAttempts;
    updateStatusBar({ status: 'waiting', position: '?' });
    showStage('waiting');
    startQueuePolling();
}

// Debug: Manually advance user status via API
async function debugAdvanceToDino() {
    const result = await apiRequest('POST', '/api/debug/advance');

    if (result.ok && !result.data.error) {
        updateStatusBar({ status: result.data.status });

        // Show appropriate stage based on status
        switch (result.data.status) {
            case 'stage1_dino':
                showStage('dino');
                startDinoGame();
                break;
            case 'stage2_captcha':
                showStage('captcha');
                break;
            case 'registering':
                showStage('register');
                break;
        }
    }
}

// Event Listeners
document.getElementById('btn-start').addEventListener('click', startSession);
document.getElementById('btn-advance-dino').addEventListener('click', debugAdvanceToDino);
document.getElementById('btn-dino-clear').addEventListener('click', () => sendDinoResult('clear'));
document.getElementById('btn-dino-fail').addEventListener('click', () => sendDinoResult('gameover'));
document.getElementById('btn-captcha-success').addEventListener('click', () => simulateCaptcha(true));
document.getElementById('btn-captcha-fail').addEventListener('click', () => simulateCaptcha(false));
document.getElementById('register-form').addEventListener('submit', submitRegistration);
document.getElementById('btn-clear-log').addEventListener('click', () => {
    document.getElementById('api-log').innerHTML = '';
});

// CAPTCHA click simulation
document.getElementById('captcha-image').addEventListener('click', (e) => {
    const rect = e.target.getBoundingClientRect();
    const x = Math.round(e.clientX - rect.left);
    const y = Math.round(e.clientY - rect.top);
    log('request', `CAPTCHA click at (${x}, ${y})`);
});

// Initialize
log('response', 'Mock Frontend initialized. Click "Start Session" to begin.');
