// Minimal JavaScript - For scroll position preservation, notifications, and agent status updates

// View Transitions API for flash notifications
let isNavigating = false;

// Function to handle page transitions with flash notification animations
function handlePageTransition(callback) {
    const notification = document.querySelector('.notification');
    
    // If View Transitions API is supported and there's a notification
    if ('startViewTransition' in document && notification && !isNavigating) {
        isNavigating = true;
        
        // Start the view transition
        document.startViewTransition(async () => {
            // Add exit animation class
            notification.style.viewTransitionName = 'notification-exit';
            
            // Wait for the transition to prepare
            await new Promise(resolve => requestAnimationFrame(resolve));
            
            // Execute the callback (navigation)
            callback();
        });
    } else {
        // Fallback: just execute the callback
        callback();
    }
}

// Save scroll position before page unload
window.addEventListener('beforeunload', function(e) {
    localStorage.setItem('mavis-scroll-position', window.scrollY);
    
    // Handle page refresh with view transitions
    const notification = document.querySelector('.notification');
    if (notification && 'startViewTransition' in document) {
        // For refresh, we can't prevent the default behavior, but we can try to add a class
        notification.style.animation = 'slideOut 0.2s ease-in forwards';
    }
});

// Handle page navigation with view transitions
document.addEventListener('DOMContentLoaded', function() {
    // Intercept navigation links when there's a notification
    const notification = document.querySelector('.notification');
    
    if (notification) {
        // Handle all navigation links
        document.querySelectorAll('a').forEach(link => {
            // Skip external links and hash links
            if (link.hostname === window.location.hostname && !link.hash) {
                link.addEventListener('click', function(e) {
                    e.preventDefault();
                    handlePageTransition(() => {
                        window.location.href = link.href;
                    });
                });
            }
        });
        
        // Handle form submissions
        document.querySelectorAll('form').forEach(form => {
            form.addEventListener('submit', function(e) {
                // Let the transition API handle the navigation
                if (!isNavigating) {
                    e.preventDefault();
                    handlePageTransition(() => {
                        form.submit();
                    });
                }
            });
        });
    }
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

    // Handle Create Agent modal directory check on Enter key
    const checkDirInput = document.getElementById('check_dir');
    const dirCheckForm = document.getElementById('dir-check-form');
    
    if (checkDirInput && dirCheckForm) {
        checkDirInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                // Submit the directory check form
                dirCheckForm.submit();
            }
        });
    }

    // Handle Create Agent form submission
    const createAgentForm = document.getElementById('create-agent-form');
    if (createAgentForm) {
        createAgentForm.addEventListener('submit', function(e) {
            // Check if branch is specified
            const branchInput = document.getElementById('branch');
            if (branchInput && branchInput.value.trim()) {
                // Show a message that the agent is being prepared
                const submitBtn = document.getElementById('create-agent-btn');
                if (submitBtn) {
                    submitBtn.textContent = 'Preparing agent...';
                    submitBtn.disabled = true;
                }
            }
        });
    }

    // Check for preparing agents and show notifications
    const checkPreparingAgents = () => {
        const preparingAgents = document.querySelectorAll('[id^="agent-preparing-"]');
        preparingAgents.forEach(agent => {
            const agentId = agent.id;
            const statusEl = agent.querySelector('.agent-status');
            if (statusEl && statusEl.textContent === 'preparing') {
                // Show a more prominent message
                if (!agent.querySelector('.preparing-message')) {
                    const message = document.createElement('div');
                    message.className = 'preparing-message';
                    message.style.cssText = 'background: #3b82f6; color: white; padding: 8px 12px; margin: 8px 0; border-radius: 6px; font-size: 14px;';
                    message.textContent = '🔄 Git repository is being prepared. You\'ll be notified when the agent is ready.';
                    agent.querySelector('.agent-task').appendChild(message);
                }
            }
        });
    };

    // Run check on page load
    checkPreparingAgents();
});

