(function() {
    var params = new URLSearchParams(window.location.search);
    var token = params.get('token');
    var syncing = false;
    var injecting = false;

    function fetchState() {
        fetch('/api/state?token=' + token)
            .then(function(resp) {
                if (!resp.ok) return null;
                return resp.json();
            })
            .then(function(state) {
                if (state) renderState(state);
            })
            .catch(function(e) {
                console.error('Failed to fetch state:', e);
            });
    }

    function renderState(state) {
        var zeroTime = '0001-01-01T00:00:00Z';
        var lastSynced = (state.sync.lastSynced && state.sync.lastSynced !== zeroTime)
            ? timeAgo(new Date(state.sync.lastSynced))
            : 'Never';
        setText('last-synced', lastSynced);
        setText('pack-count', state.sync.packCount || '\u2014');

        var badge = document.getElementById('sync-status-badge');
        if (badge) {
            if (state.sync.status === 'up_to_date') {
                badge.textContent = 'Up to Date';
                badge.className = 'badge-positive';
            } else if (state.sync.status === 'stale') {
                badge.textContent = 'Stale';
                badge.className = 'badge-warning';
            } else {
                badge.textContent = state.sync.status || 'Unknown';
                badge.className = 'badge-neutral';
            }
        }

        setText('profile-name', state.profile.name || state.profile.id);
        setText('profile-id', state.profile.id);
        setText('version', 'v' + state.version);
    }

    function setText(id, text) {
        var el = document.getElementById(id);
        if (el) el.textContent = text;
    }

    function timeAgo(date) {
        var seconds = Math.floor((Date.now() - date.getTime()) / 1000);
        if (seconds < 60) return 'just now';
        var minutes = Math.floor(seconds / 60);
        if (minutes < 60) return minutes + 'm ago';
        var hours = Math.floor(minutes / 60);
        if (hours < 24) return hours + 'h ago';
        var days = Math.floor(hours / 24);
        return days + 'd ago';
    }

    function setButtonLoading(btn, loadingText) {
        btn.classList.add('loading');
        btn.textContent = '';
        var spinner = document.createElement('span');
        spinner.className = 'spinner';
        btn.appendChild(spinner);
        btn.appendChild(document.createTextNode(loadingText));
    }

    function setButtonReady(btn, readyText) {
        btn.classList.remove('loading');
        btn.textContent = readyText;
    }

    document.addEventListener('DOMContentLoaded', function() {
        var btnSync = document.getElementById('btn-sync');
        if (btnSync) {
            btnSync.addEventListener('click', function() {
                if (syncing) return;
                syncing = true;
                setButtonLoading(btnSync, 'Syncing\u2026');
                fetch('/api/sync?token=' + token, { method: 'POST' })
                    .finally(function() {
                        setTimeout(function() {
                            syncing = false;
                            setButtonReady(btnSync, 'Sync Now');
                            fetchState();
                        }, 3000);
                    });
            });
        }

        var btnInject = document.getElementById('btn-inject');
        if (btnInject) {
            btnInject.addEventListener('click', function() {
                if (injecting) return;
                injecting = true;
                setButtonLoading(btnInject, 'Injecting\u2026');
                fetch('/api/inject?token=' + token, { method: 'POST' })
                    .finally(function() {
                        setTimeout(function() {
                            injecting = false;
                            setButtonReady(btnInject, 'Inject Now');
                            fetchState();
                        }, 3000);
                    });
            });
        }

        fetchState();
        setInterval(fetchState, 30000);
    });
})();
