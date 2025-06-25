// Minimal JavaScript - Only for scroll position preservation during auto-refresh

// Save scroll position before page unload
window.addEventListener('beforeunload', function() {
    localStorage.setItem('mavis-scroll-position', window.scrollY);
});

// Restore scroll position after page load
window.addEventListener('DOMContentLoaded', function() {
    const savedPosition = localStorage.getItem('mavis-scroll-position');
    if (savedPosition) {
        // Wait a bit for content to render, then scroll
        setTimeout(function() {
            window.scrollTo(0, parseInt(savedPosition));
        }, 100);
    }
});