(function() {
    'use strict';
    
    // Widget Configuration
    const CONFIG = {
        projectId: null,
        apiUrl: 'http://localhost:8080',
        theme: 'default',
        position: 'bottom-right',
        primaryColor: '#4f46e5',
        welcomeMessage: 'Hello! How can I help you today?'
    };

    // Widget State
    const STATE = {
        isOpen: false,
        isLoaded: false,
        messages: [],
        sessionId: null,
        userId: null,
        userName: null
    };

    // Initialize Widget
    function initWidget() {
        // Get configuration from script attributes
        const script = document.currentScript || document.querySelector('script[data-project-id]');
        if (!script) {
            console.error('Troika Chatbot: Script tag not found');
            return;
        }

        CONFIG.projectId = script.getAttribute('data-project-id');
        CONFIG.apiUrl = script.getAttribute('data-api-url') || CONFIG.apiUrl;
        
        if (!CONFIG.projectId) {
            console.error('Troika Chatbot: Project ID not specified');
            return;
        }

        // Generate session ID
        STATE.sessionId = 'session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
        
        // Create widget elements
        createWidgetElements();
        
        // Load widget styles
        loadWidgetStyles();
        
        // Mark as loaded
        STATE.isLoaded = true;
        
        console.log('Troika Chatbot: Widget initialized for project', CONFIG.projectId);
    }

    // Create Widget HTML Elements
    function createWidgetElements() {
        // Create widget container
        const widgetContainer = document.createElement('div');
        widgetContainer.id = 'troika-chatbot-widget';
        widgetContainer.innerHTML = `
            <!-- Chat Button -->
            <div id="troika-chat-button" class="troika-chat-button">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
                </svg>
            </div>

            <!-- Chat Window -->
            <div id="troika-chat-window" class="troika-chat-window">
                <div class="troika-chat-header">
                    <div class="troika-chat-title">
                        <div class="troika-chat-avatar">
                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/>
                            </svg>
                        </div>
                        <div>
                            <div class="troika-chat-bot-name">AI Assistant</div>
                            <div class="troika-chat-status">Online</div>
                        </div>
                    </div>
                    <button id="troika-chat-close" class="troika-chat-close">
                        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"/>
                            <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>

                <div id="troika-chat-messages" class="troika-chat-messages">
                    <div class="troika-message troika-message-bot">
                        <div class="troika-message-content">
                            ${CONFIG.welcomeMessage}
                        </div>
                        <div class="troika-message-time">
                            ${new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})}
                        </div>
                    </div>
                </div>

                <div class="troika-chat-input-container">
                    <input 
                        type="text" 
                        id="troika-chat-input" 
                        placeholder="Type your message..."
                        maxlength="500"
                    />
                    <button id="troika-chat-send" class="troika-chat-send">
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="22" y1="2" x2="11" y2="13"/>
                            <polygon points="22,2 15,22 11,13 2,9"/>
                        </svg>
                    </button>
                </div>

                <div class="troika-chat-footer">
                    <div class="troika-powered-by">
                        Powered by <a href="https://troikatech.com" target="_blank">Troika Tech</a>
                    </div>
                </div>
            </div>
        `;

        // Add to page
        document.body.appendChild(widgetContainer);

        // Add event listeners
        addEventListeners();
    }

    // Load Widget Styles
    function loadWidgetStyles() {
        const styles = `
            #troika-chatbot-widget {
                position: fixed;
                ${CONFIG.position.includes('right') ? 'right: 20px;' : 'left: 20px;'}
                ${CONFIG.position.includes('bottom') ? 'bottom: 20px;' : 'top: 20px;'}
                z-index: 10000;
                font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            }

            .troika-chat-button {
                width: 60px;
                height: 60px;
                border-radius: 50%;
                background: ${CONFIG.primaryColor};
                color: white;
                display: flex;
                align-items: center;
                justify-content: center;
                cursor: pointer;
                box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
                transition: all 0.3s ease;
            }

            .troika-chat-button:hover {
                transform: scale(1.1);
                box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
            }

            .troika-chat-window {
                width: 350px;
                height: 500px;
                background: white;
                border-radius: 12px;
                box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
                display: none;
                flex-direction: column;
                overflow: hidden;
                margin-bottom: 10px;
            }

            .troika-chat-window.open {
                display: flex;
            }

            .troika-chat-header {
                background: ${CONFIG.primaryColor};
                color: white;
                padding: 16px;
                display: flex;
                align-items: center;
                justify-content: space-between;
            }

            .troika-chat-title {
                display: flex;
                align-items: center;
                gap: 12px;
            }

            .troika-chat-avatar {
                width: 36px;
                height: 36px;
                border-radius: 50%;
                background: rgba(255, 255, 255, 0.2);
                display: flex;
                align-items: center;
                justify-content: center;
            }

            .troika-chat-bot-name {
                font-weight: 600;
                font-size: 14px;
            }

            .troika-chat-status {
                font-size: 12px;
                opacity: 0.8;
            }

            .troika-chat-close {
                background: none;
                border: none;
                color: white;
                cursor: pointer;
                padding: 4px;
                border-radius: 4px;
                opacity: 0.8;
                transition: opacity 0.2s;
            }

            .troika-chat-close:hover {
                opacity: 1;
            }

            .troika-chat-messages {
                flex: 1;
                overflow-y: auto;
                padding: 16px;
                display: flex;
                flex-direction: column;
                gap: 12px;
            }

            .troika-message {
                max-width: 80%;
                animation: fadeIn 0.3s ease;
            }

            .troika-message-bot {
                align-self: flex-start;
            }

            .troika-message-user {
                align-self: flex-end;
            }

            .troika-message-content {
                padding: 12px 16px;
                border-radius: 18px;
                font-size: 14px;
                line-height: 1.4;
                word-wrap: break-word;
            }

            .troika-message-bot .troika-message-content {
                background: #f1f3f4;
                color: #333;
            }

            .troika-message-user .troika-message-content {
                background: ${CONFIG.primaryColor};
                color: white;
            }

            .troika-message-time {
                font-size: 11px;
                color: #999;
                margin-top: 4px;
                text-align: center;
            }

            .troika-chat-input-container {
                display: flex;
                padding: 16px;
                gap: 8px;
                border-top: 1px solid #e5e7eb;
            }

            #troika-chat-input {
                flex: 1;
                padding: 12px 16px;
                border: 1px solid #e5e7eb;
                border-radius: 24px;
                font-size: 14px;
                outline: none;
                transition: border-color 0.2s;
            }

            #troika-chat-input:focus {
                border-color: ${CONFIG.primaryColor};
            }

            .troika-chat-send {
                width: 44px;
                height: 44px;
                border-radius: 50%;
                background: ${CONFIG.primaryColor};
                color: white;
                border: none;
                cursor: pointer;
                display: flex;
                align-items: center;
                justify-content: center;
                transition: all 0.2s;
            }

            .troika-chat-send:hover {
                transform: scale(1.05);
            }

            .troika-chat-send:disabled {
                opacity: 0.5;
                cursor: not-allowed;
                transform: none;
            }

            .troika-chat-footer {
                padding: 8px 16px;
                border-top: 1px solid #e5e7eb;
                background: #f9fafb;
            }

            .troika-powered-by {
                font-size: 11px;
                color: #6b7280;
                text-align: center;
            }

            .troika-powered-by a {
                color: ${CONFIG.primaryColor};
                text-decoration: none;
            }

            .troika-typing-indicator {
                display: flex;
                align-items: center;
                gap: 4px;
                padding: 12px 16px;
                background: #f1f3f4;
                border-radius: 18px;
                max-width: 80px;
            }

            .troika-typing-dot {
                width: 6px;
                height: 6px;
                border-radius: 50%;
                background: #999;
                animation: typingAnimation 1.4s infinite;
            }

            .troika-typing-dot:nth-child(2) {
                animation-delay: 0.2s;
            }

            .troika-typing-dot:nth-child(3) {
                animation-delay: 0.4s;
            }

            @keyframes fadeIn {
                from { opacity: 0; transform: translateY(10px); }
                to { opacity: 1; transform: translateY(0); }
            }

            @keyframes typingAnimation {
                0%, 60%, 100% { opacity: 0.3; }
                30% { opacity: 1; }
            }

            @media (max-width: 480px) {
                .troika-chat-window {
                    width: calc(100vw - 40px);
                    height: calc(100vh - 100px);
                    position: fixed;
                    top: 20px;
                    left: 20px;
                    right: 20px;
                    bottom: 80px;
                    margin: 0;
                }
            }
        `;

        const styleSheet = document.createElement('style');
        styleSheet.textContent = styles;
        document.head.appendChild(styleSheet);
    }

    // Add Event Listeners
    function addEventListeners() {
        const chatButton = document.getElementById('troika-chat-button');
        const chatWindow = document.getElementById('troika-chat-window');
        const chatClose = document.getElementById('troika-chat-close');
        const chatInput = document.getElementById('troika-chat-input');
        const chatSend = document.getElementById('troika-chat-send');

        // Toggle chat window
        chatButton.addEventListener('click', toggleChat);
        chatClose.addEventListener('click', closeChat);

        // Send message
        chatSend.addEventListener('click', sendMessage);
        chatInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });

        // Auto-resize input
        chatInput.addEventListener('input', (e) => {
            chatSend.disabled = !e.target.value.trim();
        });
    }

    // Toggle Chat Window
    function toggleChat() {
        const chatWindow = document.getElementById('troika-chat-window');
        STATE.isOpen = !STATE.isOpen;
        
        if (STATE.isOpen) {
            chatWindow.classList.add('open');
            document.getElementById('troika-chat-input').focus();
        } else {
            chatWindow.classList.remove('open');
        }
    }

    // Close Chat Window
    function closeChat() {
        const chatWindow = document.getElementById('troika-chat-window');
        STATE.isOpen = false;
        chatWindow.classList.remove('open');
    }

    // Send Message
    async function sendMessage() {
        const input = document.getElementById('troika-chat-input');
        const message = input.value.trim();
        
        if (!message) return;

        // Clear input
        input.value = '';
        document.getElementById('troika-chat-send').disabled = true;

        // Add user message to chat
        addMessage(message, 'user');

        // Show typing indicator
        showTypingIndicator();

        try {
            // Send to API
            const response = await fetch(`${CONFIG.apiUrl}/api/projects/${CONFIG.projectId}/chat`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    message: message,
                    session_id: STATE.sessionId,
                    user_id: STATE.userId || 'anonymous',
                    user_name: STATE.userName || 'User'
                })
            });

            const data = await response.json();

            // Hide typing indicator
            hideTypingIndicator();

            if (data.status === 'success') {
                addMessage(data.response, 'bot');
            } else {
                addMessage('Sorry, I encountered an error. Please try again.', 'bot');
            }

        } catch (error) {
            hideTypingIndicator();
            console.error('Chat error:', error);
            addMessage('Sorry, I\'m having trouble connecting. Please try again later.', 'bot');
        }
    }

    // Add Message to Chat
    function addMessage(content, sender) {
        const messagesContainer = document.getElementById('troika-chat-messages');
        const messageDiv = document.createElement('div');
        messageDiv.className = `troika-message troika-message-${sender}`;
        
        const currentTime = new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
        
        messageDiv.innerHTML = `
            <div class="troika-message-content">${content}</div>
            <div class="troika-message-time">${currentTime}</div>
        `;

        messagesContainer.appendChild(messageDiv);
        messagesContainer.scrollTop = messagesContainer.scrollHeight;

        // Store message
        STATE.messages.push({
            content: content,
            sender: sender,
            timestamp: new Date()
        });
    }

    // Show Typing Indicator
    function showTypingIndicator() {
        const messagesContainer = document.getElementById('troika-chat-messages');
        const typingDiv = document.createElement('div');
        typingDiv.id = 'troika-typing-indicator';
        typingDiv.className = 'troika-message troika-message-bot';
        typingDiv.innerHTML = `
            <div class="troika-typing-indicator">
                <div class="troika-typing-dot"></div>
                <div class="troika-typing-dot"></div>
                <div class="troika-typing-dot"></div>
            </div>
        `;

        messagesContainer.appendChild(typingDiv);
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }

    // Hide Typing Indicator
    function hideTypingIndicator() {
        const typingIndicator = document.getElementById('troika-typing-indicator');
        if (typingIndicator) {
            typingIndicator.remove();
        }
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initWidget);
    } else {
        initWidget();
    }

})();
