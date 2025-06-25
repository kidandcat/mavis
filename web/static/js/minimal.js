// Minimal JavaScript - Only for scroll position preservation during auto-refresh

// Save scroll position before page unload
window.addEventListener('beforeunload', function() {
    localStorage.setItem('mavis-scroll-position', window.scrollY);
});

// Handle restart button - redirect to root before restarting
document.addEventListener('DOMContentLoaded', function() {
    // Find the restart form by its action attribute
    const restartForm = document.querySelector('form[action="/api/system/restart"]');
    
    if (restartForm) {
        restartForm.addEventListener('submit', function(e) {
            e.preventDefault();
            
            // Redirect to root path
            window.location.href = '/';
            
            // Wait 1 second then submit the restart request
            setTimeout(function() {
                fetch('/api/system/restart', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    }
                });
            }, 1000);
        });
    }
});

