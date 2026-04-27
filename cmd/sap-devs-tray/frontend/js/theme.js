/**
 * Theme controller — shared between dashboard (index.html) and config editor (config.html).
 * Reads the user's theme preference from /api/config and applies data-theme / data-dark
 * attributes on <body>. Dark mode tracks the OS preference via matchMedia.
 *
 * Load this BEFORE app.js / config.js so attributes are set before first paint.
 */
(function() {
    var params = new URLSearchParams(window.location.search);
    var token = params.get('token');
    var darkMQ = window.matchMedia('(prefers-color-scheme: dark)');

    function applyDark() {
        if (darkMQ.matches) {
            document.body.setAttribute('data-dark', '');
        } else {
            document.body.removeAttribute('data-dark');
        }
    }

    function applyTheme(name) {
        if (name && name !== 'standard') {
            document.body.setAttribute('data-theme', name);
        } else {
            document.body.removeAttribute('data-theme');
        }
    }

    applyDark();
    darkMQ.addEventListener('change', applyDark);

    if (token) {
        fetch('/api/config?token=' + token)
            .then(function(r) { return r.ok ? r.json() : null; })
            .then(function(cfg) {
                if (cfg) applyTheme(cfg.tray_theme || 'joule');
            })
            .catch(function() {});
    }

    window.__sapDevsTheme = {
        apply: applyTheme
    };
})();
