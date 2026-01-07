export default {
    name: 'CacheManager',
    data() {
        return {
            stats: {
                total_size: 0,
                human_size: '0 B'
            },
            settings: {
                enabled: false,
                max_age_days: 30,
                max_size_mb: 1024,
                cleanup_interval_mins: 60
            },
            loading: false,
            saving: false
        }
    },
    methods: {
        async loadStats() {
            this.loading = true;
            try {
                const { data } = await window.http.get('/api/cache/stats');
                if (data.results) {
                    this.stats = data.results;
                }
            } catch (err) {
                console.error('Failed to load cache stats', err);
            } finally {
                this.loading = false;
            }
        },
        async loadSettings() {
            try {
                const { data } = await window.http.get('/api/cache/settings');
                if (data.results) {
                    this.settings = data.results;
                }
            } catch (err) {
                console.error('Failed to load cache settings', err);
            }
        },
        async saveSettings() {
            this.saving = true;
            try {
                await window.http.put('/api/cache/settings', this.settings);
                window.showSuccessInfo('Cache settings saved. Background task updated.');
                $('#modalCacheSettings').modal('hide');
            } catch (err) {
                console.error('Failed to save cache settings', err);
                window.showErrorInfo('Failed to save settings');
            } finally {
                this.saving = false;
            }
        },
        openSettings() {
            this.loadSettings();
            $('#modalCacheSettings').modal('show');
        },
        async clearCache() {
            if (!confirm('Are you sure you want to clear the global cache? This will delete all downloaded media, QR codes, and temporary files.')) {
                return;
            }
            this.loading = true;
            try {
                await window.http.post('/api/cache/clear');
                window.showSuccessInfo('Global cache cleared successfully');
                await this.loadStats();
            } catch (err) {
                console.error('Failed to clear global cache', err);
                window.showErrorInfo('Failed to clear cache');
            } finally {
                this.loading = false;
            }
        }
    },
    mounted() {
        this.loadStats();
    },
    template: `
        <div class="card">
            <div class="content">
                <div class="header">
                    <i class="hdd outline icon"></i>
                    Cache System
                </div>
                <div class="meta">Automatic Maintenance</div>
                <div class="description center aligned" style="padding: 1.5em 0;">
                    <div class="ui mini statistic">
                        <div class="value">
                            {{ stats.human_size }}
                        </div>
                        <div class="label">
                            Total Used
                        </div>
                    </div>
                </div>
            </div>
            <div class="extra content">
                <div class="ui three buttons">
                    <button class="ui basic blue button" @click="loadStats" :class="{ loading: loading }" title="Refresh Stats">
                        <i class="refresh icon"></i>
                    </button>
                    <button class="ui basic grey button" @click="openSettings" title="Settings">
                        <i class="cog icon"></i>
                    </button>
                    <button class="ui basic red button" @click="clearCache" :class="{ loading: loading }" title="Clear Everything">
                        <i class="trash icon"></i>
                    </button>
                </div>
            </div>

            <!-- Settings Modal -->
            <div class="ui modal" id="modalCacheSettings">
                <i class="close icon"></i>
                <div class="header">
                    <i class="settings icon"></i> Cache & Storage Settings
                </div>
                <div class="content">
                    <div class="ui form">
                        <div class="inline field">
                            <div class="ui toggle checkbox">
                                <input type="checkbox" v-model="settings.enabled">
                                <label>Enable Automatic Background Cleanup</label>
                            </div>
                        </div>
                        <div class="two fields">
                            <div class="field">
                                <label>Max File Age (Days)</label>
                                <input type="number" v-model.number="settings.max_age_days" placeholder="30">
                                <small>Delete files older than this</small>
                            </div>
                            <div class="field">
                                <label>Max Cache Size (MB)</label>
                                <input type="number" v-model.number="settings.max_size_mb" placeholder="1024">
                                <small>Delete oldest files if total size exceeds this</small>
                            </div>
                        </div>
                        <div class="field">
                            <label>Check Interval (Minutes)</label>
                            <input type="number" v-model.number="settings.cleanup_interval_mins" placeholder="60">
                            <small>How often to run the maintenance task</small>
                        </div>
                    </div>
                </div>
                <div class="actions">
                    <div class="ui negative button">Cancel</div>
                    <div class="ui positive right labeled icon button" @click="saveSettings" :class="{ loading: saving }">
                        Save Configuration
                        <i class="checkmark icon"></i>
                    </div>
                </div>
            </div>
        </div>
    `
};
