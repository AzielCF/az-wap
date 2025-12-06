/**
 * Funciones auxiliares para InstanceManager
 */

export function showSuccessInfo(message) {
    if (typeof window.showSuccessInfo === 'function') {
        window.showSuccessInfo(message);
    }
}

export function showErrorInfo(message) {
    if (typeof window.showErrorInfo === 'function') {
        window.showErrorInfo(message);
    }
}

export function chatwootWebhookUrl(instanceId) {
    if (!instanceId) return '';
    const base = window.location ? `${window.location.protocol}//${window.location.host}` : '';
    const basePath = window.AppBasePath || '';
    const normalizedBasePath = basePath && basePath !== '/' ? basePath.replace(/\/$/, '') : '';
    return `${base}${normalizedBasePath}/instances/${instanceId}/chatwoot/webhook`;
}

export function botWebhookUrl(botId) {
    if (!botId) return '';
    const base = window.location ? `${window.location.protocol}//${window.location.host}` : '';
    const basePath = window.AppBasePath || '';
    const normalizedBasePath = basePath && basePath !== '/' ? basePath.replace(/\/$/, '') : '';
    return `${base}${normalizedBasePath}/bots/${botId}/webhook`;
}

export function copyToClipboard(text, successMessage) {
    if (!text) return;
    if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text)
            .then(() => showSuccessInfo(successMessage || 'Copied to clipboard.'))
            .catch((err) => showErrorInfo(err?.message || 'Failed to copy to clipboard'));
    }
}

export function filterCredentialsByKind(credentials, kind) {
    if (!Array.isArray(credentials)) return [];
    return credentials.filter((c) => c && c.kind === kind);
}

export function handleApiError(err, defaultMessage) {
    const message = err?.response?.data?.message || err.message || defaultMessage;
    showErrorInfo(message);
}
