(function() {
    var params = new URLSearchParams(window.location.search);
    var token = params.get('token');
    var typeaheadTimer = null;
    var lastCityResults = [];

    function api(path, opts) {
        opts = opts || {};
        var sep = path.indexOf('?') >= 0 ? '&' : '?';
        var url = path + sep + 'token=' + token;
        return fetch(url, opts).then(function(r) {
            if (!r.ok && r.status !== 400) throw new Error('HTTP ' + r.status);
            return r.json();
        });
    }

    window.togglePanel = function(headerEl) {
        var panel = headerEl.parentElement;
        panel.classList.toggle('collapsed');
    };

    function init() {
        Promise.all([
            api('/api/config'),
            api('/api/languages'),
            api('/api/service-status')
        ]).then(function(results) {
            populateForm(results[0]);
            populateLanguages(results[1], results[0].language);
            populateServiceStatus(results[2]);
        }).catch(function(e) {
            console.error('Failed to load config:', e);
        });
    }

    function populateLanguages(languages, current) {
        var sel = document.getElementById('cfg-language');
        sel.textContent = '';
        for (var i = 0; i < languages.length; i++) {
            var opt = document.createElement('option');
            opt.value = languages[i].code;
            opt.textContent = languages[i].label;
            if (languages[i].code === current) opt.selected = true;
            sel.appendChild(opt);
        }
    }

    function populateForm(cfg) {
        setVal('cfg-location', cfg.location);
        setVal('cfg-experience', cfg.experience_level);
        setVal('cfg-company-repo', cfg.company_repo);
        setVal('cfg-tip-rotation', cfg.tip_rotation);
        setChecked('cfg-tutorial-interactive', cfg.tutorial_interactive);
        setVal('cfg-local-radius', cfg.events_local_radius);
        setVal('cfg-regional-radius', cfg.events_regional_radius);
        setVal('cfg-notify-days', cfg.events_notify_days);
        setVal('cfg-notify-method', cfg.events_notify_method);
        setChecked('cfg-sync-disabled', cfg.sync_disabled);
        setVal('cfg-sync-tips', cfg.sync_tips);
        setVal('cfg-sync-tools', cfg.sync_tools);
        setVal('cfg-sync-resources', cfg.sync_resources);
        setVal('cfg-sync-context', cfg.sync_context);
        setVal('cfg-sync-events', cfg.sync_events);
        setVal('cfg-sync-youtube', cfg.sync_youtube);
        setVal('cfg-sync-discovery', cfg.sync_discovery);
        setVal('cfg-sync-tutorials', cfg.sync_tutorials);
        setVal('cfg-sync-advocates', cfg.sync_advocates);
        setVal('cfg-sync-mcp', cfg.sync_mcp);
        setVal('cfg-sync-learning', cfg.sync_learning);
        setVal('cfg-service-interval', cfg.service_interval);
    }

    function setVal(id, val) {
        var el = document.getElementById(id);
        if (el) el.value = val != null ? val : '';
    }

    function setChecked(id, val) {
        var el = document.getElementById(id);
        if (el) el.checked = !!val;
    }

    function populateServiceStatus(status) {
        var schBadge = document.getElementById('scheduler-badge');
        var schInstalled = document.getElementById('scheduler-installed');
        var schNotInstalled = document.getElementById('scheduler-not-installed');

        if (status.scheduler.installed) {
            schBadge.textContent = 'Installed';
            schBadge.className = 'cfg-badge installed';
            schInstalled.classList.remove('hidden');
            schNotInstalled.classList.add('hidden');
        } else {
            schBadge.textContent = 'Not Installed';
            schBadge.className = 'cfg-badge not-installed';
            schInstalled.classList.add('hidden');
            schNotInstalled.classList.remove('hidden');
        }

        var asBadge = document.getElementById('autostart-badge');
        var asInstalled = document.getElementById('autostart-installed');
        var asNotInstalled = document.getElementById('autostart-not-installed');

        if (status.autostart.installed) {
            asBadge.textContent = 'Installed';
            asBadge.className = 'cfg-badge installed';
            asInstalled.classList.remove('hidden');
            asNotInstalled.classList.add('hidden');
        } else {
            asBadge.textContent = 'Not Installed';
            asBadge.className = 'cfg-badge not-installed';
            asInstalled.classList.add('hidden');
            asNotInstalled.classList.remove('hidden');
        }
    }

    function refreshServiceStatus() {
        api('/api/service-status').then(populateServiceStatus);
    }

    function setupTypeahead() {
        var input = document.getElementById('cfg-location');
        var list = document.getElementById('cfg-location-list');

        input.addEventListener('input', function() {
            clearTimeout(typeaheadTimer);
            var q = input.value.trim();
            if (q.length < 2) {
                list.classList.add('hidden');
                return;
            }
            typeaheadTimer = setTimeout(function() {
                api('/api/cities?q=' + encodeURIComponent(q)).then(function(cities) {
                    lastCityResults = cities;
                    list.textContent = '';
                    if (!cities || cities.length === 0) {
                        list.classList.add('hidden');
                        return;
                    }
                    for (var i = 0; i < cities.length; i++) {
                        var item = document.createElement('div');
                        item.className = 'cfg-typeahead-item';
                        item.textContent = cities[i].name + ', ' + cities[i].country;
                        item.dataset.value = cities[i].name + ', ' + cities[i].country;
                        item.addEventListener('click', function() {
                            input.value = this.dataset.value;
                            list.classList.add('hidden');
                            checkLocationWarning();
                        });
                        list.appendChild(item);
                    }
                    list.classList.remove('hidden');
                });
            }, 200);
        });

        input.addEventListener('blur', function() {
            setTimeout(function() { list.classList.add('hidden'); }, 150);
            checkLocationWarning();
        });
    }

    function checkLocationWarning() {
        var input = document.getElementById('cfg-location');
        var warn = document.getElementById('cfg-location-warning');
        var val = input.value.trim();
        if (!val) {
            warn.classList.add('hidden');
            return;
        }
        var found = false;
        for (var i = 0; i < lastCityResults.length; i++) {
            var match = lastCityResults[i].name + ', ' + lastCityResults[i].country;
            if (match.toLowerCase() === val.toLowerCase()) { found = true; break; }
        }
        if (!found) {
            api('/api/cities?q=' + encodeURIComponent(val.split(',')[0])).then(function(cities) {
                var matched = false;
                for (var i = 0; i < cities.length; i++) {
                    var m = cities[i].name + ', ' + cities[i].country;
                    if (m.toLowerCase() === val.toLowerCase()) { matched = true; break; }
                }
                if (matched) warn.classList.add('hidden');
                else warn.classList.remove('hidden');
            });
        } else {
            warn.classList.add('hidden');
        }
    }

    function setupDetect() {
        var btn = document.getElementById('btn-detect-location');
        btn.addEventListener('click', function() {
            btn.disabled = true;
            btn.textContent = 'Detecting...';
            api('/api/detect-location', { method: 'POST' })
                .then(function(data) {
                    if (data.error) {
                        showSaveStatus(data.error, 'error');
                    } else {
                        document.getElementById('cfg-location').value = data.city + ', ' + data.country;
                        document.getElementById('cfg-location-warning').classList.add('hidden');
                    }
                })
                .catch(function() {
                    showSaveStatus('Could not detect location', 'error');
                })
                .finally(function() {
                    btn.disabled = false;
                    btn.textContent = 'Detect';
                });
        });
    }

    function validateForm() {
        clearErrors();
        var errors = {};

        var companyRepo = getVal('cfg-company-repo');
        if (companyRepo) {
            try {
                var u = new URL(companyRepo);
                if (u.protocol !== 'https:') errors['company_repo'] = 'Must be a valid URL (https://...)';
            } catch(e) {
                errors['company_repo'] = 'Must be a valid URL (https://...)';
            }
        }

        var intFields = [
            { id: 'cfg-local-radius', key: 'events_local_radius' },
            { id: 'cfg-regional-radius', key: 'events_regional_radius' },
            { id: 'cfg-notify-days', key: 'events_notify_days' }
        ];
        for (var i = 0; i < intFields.length; i++) {
            var v = parseInt(getVal(intFields[i].id), 10);
            if (isNaN(v) || v <= 0) errors[intFields[i].key] = 'Must be greater than 0';
        }

        var durationFields = [
            'sync_tips', 'sync_tools', 'sync_advocates', 'sync_resources',
            'sync_context', 'sync_mcp', 'sync_events', 'sync_youtube',
            'sync_discovery', 'sync_tutorials', 'sync_learning', 'service_interval'
        ];
        var durationRe = /^(\d+(\.\d+)?(h|m|s|ms|us|µs|ns))+$/;
        for (var j = 0; j < durationFields.length; j++) {
            var dv = getVal('cfg-' + durationFields[j].replace(/_/g, '-'));
            if (!dv || !durationRe.test(dv)) {
                errors[durationFields[j]] = 'Invalid duration format';
            }
        }

        return errors;
    }

    function clearErrors() {
        var errEls = document.querySelectorAll('.cfg-error');
        for (var i = 0; i < errEls.length; i++) errEls[i].classList.add('hidden');
        var inputs = document.querySelectorAll('.fd-input.has-error');
        for (var j = 0; j < inputs.length; j++) inputs[j].classList.remove('has-error');
    }

    function showErrors(errors) {
        var fieldMap = {
            company_repo: 'cfg-company-repo',
            events_local_radius: 'cfg-local-radius',
            events_regional_radius: 'cfg-regional-radius',
            events_notify_days: 'cfg-notify-days',
            sync_tips: 'cfg-sync-tips', sync_tools: 'cfg-sync-tools',
            sync_advocates: 'cfg-sync-advocates', sync_resources: 'cfg-sync-resources',
            sync_context: 'cfg-sync-context', sync_mcp: 'cfg-sync-mcp',
            sync_events: 'cfg-sync-events', sync_youtube: 'cfg-sync-youtube',
            sync_discovery: 'cfg-sync-discovery', sync_tutorials: 'cfg-sync-tutorials',
            sync_learning: 'cfg-sync-learning', service_interval: 'cfg-service-interval'
        };
        for (var key in errors) {
            var inputId = fieldMap[key];
            if (!inputId) continue;
            var input = document.getElementById(inputId);
            if (input) input.classList.add('has-error');
            var errEl = document.getElementById(inputId + '-error');
            if (errEl) {
                errEl.textContent = errors[key];
                errEl.classList.remove('hidden');
            }
        }
    }

    function setupSave() {
        document.getElementById('btn-save').addEventListener('click', function() {
            var errors = validateForm();
            if (Object.keys(errors).length > 0) {
                showErrors(errors);
                showSaveStatus('Please fix the errors above', 'error');
                return;
            }

            var body = {
                language: getVal('cfg-language'),
                location: getVal('cfg-location'),
                experience_level: getVal('cfg-experience'),
                company_repo: getVal('cfg-company-repo'),
                tip_rotation: getVal('cfg-tip-rotation'),
                tutorial_interactive: document.getElementById('cfg-tutorial-interactive').checked,
                events_local_radius: parseInt(getVal('cfg-local-radius'), 10),
                events_regional_radius: parseInt(getVal('cfg-regional-radius'), 10),
                events_notify_days: parseInt(getVal('cfg-notify-days'), 10),
                events_notify_method: getVal('cfg-notify-method'),
                sync_disabled: document.getElementById('cfg-sync-disabled').checked,
                sync_tips: getVal('cfg-sync-tips'),
                sync_tools: getVal('cfg-sync-tools'),
                sync_advocates: getVal('cfg-sync-advocates'),
                sync_resources: getVal('cfg-sync-resources'),
                sync_context: getVal('cfg-sync-context'),
                sync_mcp: getVal('cfg-sync-mcp'),
                sync_events: getVal('cfg-sync-events'),
                sync_youtube: getVal('cfg-sync-youtube'),
                sync_discovery: getVal('cfg-sync-discovery'),
                sync_tutorials: getVal('cfg-sync-tutorials'),
                sync_learning: getVal('cfg-sync-learning'),
                service_interval: getVal('cfg-service-interval'),
                tray_autostart: false
            };

            api('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            }).then(function(data) {
                if (data.errors) {
                    showErrors(data.errors);
                    showSaveStatus('Validation failed', 'error');
                } else {
                    showSaveStatus('Configuration saved', 'success');
                    setTimeout(function() { hideSaveStatus(); }, 3000);
                }
            }).catch(function(e) {
                showSaveStatus('Save failed: ' + e.message, 'error');
            });
        });
    }

    function showSaveStatus(msg, type) {
        var el = document.getElementById('cfg-save-status');
        el.textContent = msg;
        el.className = 'cfg-save-status ' + type;
        el.classList.remove('hidden');
    }

    function hideSaveStatus() {
        document.getElementById('cfg-save-status').classList.add('hidden');
    }

    function getVal(id) {
        var el = document.getElementById(id);
        return el ? el.value : '';
    }

    function setupServiceActions() {
        bindAction('btn-scheduler-install', '/api/service-install', 'Installing...');
        bindAction('btn-scheduler-uninstall', '/api/service-uninstall', 'Uninstalling...');
        bindAction('btn-autostart-install', '/api/autostart-install', 'Installing...');
        bindAction('btn-autostart-uninstall', '/api/autostart-uninstall', 'Uninstalling...');
    }

    function bindAction(btnId, endpoint, loadingText) {
        var btn = document.getElementById(btnId);
        if (!btn) return;
        btn.addEventListener('click', function() {
            var origText = btn.textContent;
            btn.disabled = true;
            btn.textContent = loadingText;
            api(endpoint, { method: 'POST' })
                .then(function(data) {
                    if (data.error) {
                        showSaveStatus(data.error, 'error');
                    }
                    refreshServiceStatus();
                })
                .catch(function(e) {
                    showSaveStatus('Action failed: ' + e.message, 'error');
                })
                .finally(function() {
                    btn.disabled = false;
                    btn.textContent = origText;
                });
        });
    }

    document.addEventListener('DOMContentLoaded', function() {
        setupTypeahead();
        setupDetect();
        setupSave();
        setupServiceActions();
        init();
    });
})();
