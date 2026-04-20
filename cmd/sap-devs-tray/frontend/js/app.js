(function() {
    var params = new URLSearchParams(window.location.search);
    var token = params.get('token');
    var syncing = false;
    var injecting = false;
    var pollTimer = null;

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

        var nextSync = (state.sync.nextSync && state.sync.nextSync !== zeroTime)
            ? timeUntil(new Date(state.sync.nextSync))
            : '\u2014';
        setText('next-sync', nextSync);

        setText('pack-count', state.sync.packCount || '\u2014');

        var badge = document.getElementById('sync-status-badge');
        if (badge) {
            if (syncing) {
                badge.textContent = 'Syncing\u2026';
                badge.className = 'badge-neutral';
            } else if (state.sync.status === 'up_to_date') {
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

        var profileName = state.profile.name || state.profile.id;
        setText('profile-name', profileName);

        var avatar = document.getElementById('profile-avatar');
        if (avatar) {
            avatar.textContent = profileName.charAt(0).toUpperCase();
        }

        var packs = state.profile.packs;
        setText('profile-packs', packs && packs.length ? packs.join(', ') : '\u2014');

        var ver = state.version;
        setText('version', ver.charAt(0) === 'v' ? ver : 'v' + ver);

        renderTools(state.tools);
    }

    function renderTools(tools) {
        var container = document.getElementById('tools-list');
        if (!container || !tools) return;
        container.textContent = '';
        for (var i = 0; i < tools.length; i++) {
            var t = tools[i];
            var row = document.createElement('div');
            row.className = 'tool-row';

            var name = document.createElement('span');
            name.className = 'tool-name';
            name.textContent = t.name;

            var status = document.createElement('span');
            var dot = document.createElement('span');
            dot.className = 'dot';
            if (t.injected) {
                status.className = 'tool-status injected';
                status.appendChild(dot);
                status.appendChild(document.createTextNode('Injected'));
            } else {
                status.className = 'tool-status not-detected';
                status.appendChild(dot);
                status.appendChild(document.createTextNode('Not detected'));
            }

            row.appendChild(name);
            row.appendChild(status);
            container.appendChild(row);
        }
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

    function timeUntil(date) {
        var seconds = Math.floor((date.getTime() - Date.now()) / 1000);
        if (seconds <= 0) return 'now';
        var minutes = Math.floor(seconds / 60);
        if (minutes < 60) return minutes + 'm';
        var hours = Math.floor(minutes / 60);
        if (hours < 24) return hours + 'h';
        var days = Math.floor(hours / 24);
        return days + 'd';
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

    function syncComplete() {
        syncing = false;
        preSyncTime = null;
        if (pollTimer) {
            clearInterval(pollTimer);
            pollTimer = null;
        }
        var btn = document.getElementById('btn-sync');
        if (btn) setButtonReady(btn, '\u21BB Sync Now');
        fetchState();
        fetchSyncLog();
    }

    function fetchSyncLog() {
        fetch('/api/sync-log?token=' + token)
            .then(function(r) { return r.json(); })
            .then(function(data) {
                var logEl = document.getElementById('sync-log');
                var logText = document.getElementById('sync-log-text');
                if (!logEl || !logText) return;
                if (data.log) {
                    logEl.classList.remove('hidden');
                    logText.textContent = data.log.trim();
                    logEl.scrollTop = logEl.scrollHeight;
                }
                if (syncing && !data.running) {
                    syncComplete();
                }
            })
            .catch(function() {});
    }

    document.addEventListener('DOMContentLoaded', function() {
        var btnClose = document.getElementById('btn-close');
        if (btnClose) {
            btnClose.addEventListener('click', function() {
                fetch('/api/hide?token=' + token, { method: 'POST' });
            });
        }

        var btnSync = document.getElementById('btn-sync');
        if (btnSync) {
            btnSync.addEventListener('click', function() {
                if (syncing) return;
                syncing = true;
                var logEl = document.getElementById('sync-log');
                if (logEl) logEl.classList.add('hidden');
                fetchState();
                setButtonLoading(btnSync, 'Syncing\u2026');
                fetch('/api/sync?token=' + token, { method: 'POST' });
                pollTimer = setInterval(function() {
                    fetchSyncLog();
                }, 2000);
                setTimeout(function() {
                    if (syncing) syncComplete();
                }, 120000);
            });
        }

        var btnInject = document.getElementById('btn-inject');
        if (btnInject) {
            btnInject.addEventListener('click', function() {
                if (injecting) return;
                injecting = true;
                setButtonLoading(btnInject, 'Injecting\u2026');
                fetch('/api/inject?token=' + token, { method: 'POST' });
                setTimeout(function() {
                    injecting = false;
                    setButtonReady(btnInject, '\u21BB Inject Now');
                    fetchState();
                }, 5000);
            });
        }

        fetchState();
        setInterval(fetchState, 30000);
    });
})();
