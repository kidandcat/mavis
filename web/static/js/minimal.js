// Minimal JavaScript - Only for scroll position preservation during auto-refresh

// Save scroll position before page unload
window.addEventListener('beforeunload', function() {
    localStorage.setItem('mavis-scroll-position', window.scrollY);
});

// Handle restart button - submit restart request then redirect
document.addEventListener('DOMContentLoaded', function() {
    // Find the restart form by its action attribute
    const restartForm = document.querySelector('form[action="/api/system/restart"]');
    
    if (restartForm) {
        restartForm.addEventListener('submit', function(e) {
            e.preventDefault();
            
            // Submit the restart request first
            fetch('/api/system/restart', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                }
            }).then(() => {
                // After request is sent, redirect to root
                window.location.href = '/';
            }).catch(() => {
                // Even on error, redirect to root
                window.location.href = '/';
            });
        });
    }
});

