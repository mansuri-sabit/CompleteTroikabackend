// static/widget.js
(function() {
    'use strict';
    
    window.TroikaChatbot = {
        init: function(config) {
            console.log('🚀 Troika Chatbot initializing...', config);
            
            // Default configuration
            var defaultConfig = {
                theme: 'light',
                position: 'bottom-right',
                primaryColor: '#667eea',
                welcomeMessage: 'Hello! How can I help you today?',
                placeholder: 'Type your message...',
                height: '500px',
                width: '350px',
                apiUrl: 'https://completetroikabackend.onrender.com/api'
            };
            
            // Merge configurations
            config = Object.assign(defaultConfig, config);
            
            // Create widget HTML
            this.createWidget(config);
        },
        
        createWidget: function(config) {
            // Remove existing widget if any
            var existingWidget = document.getElementById('troika-widget-' + config.projectId);
            if (existingWidget) {
                existingWidget.remove();
            }
            
            // Create widget container
            var widgetContainer = document.createElement('div');
            widgetContainer.id = 'troika-widget-' + config.projectId;
            widgetContainer.innerHTML = this.getWidgetHTML(config);
            
            // Add styles
            this.addStyles(config);
            
            // Append to body
            document.body.appendChild(widgetContainer);
            
            // Add event listeners
            this.addEventListeners(config);
            
            console.log('✅ Troika Chatbot widget created successfully');
        },
        
        getWidgetHTML: function(config) {
            var position = this.getPositionStyles(config.position);
            
            return `
                <div class="troika-chatbot-container" style="${position}">
                    <!-- Chat Button -->
                    <div class="troika-chat-button" style="
                        width: 60px; 
                        height: 60px; 
                        background: ${config.primaryColor}; 
                        border-radius: 50%; 
                        cursor: pointer; 
                        display: flex; 
                        align-items: center; 
                        justify-content: center; 
                        color: white; 
                        font-size: 24px; 
                        box-shadow: 0 4px 20px rgba(0,0,0,0.2);
                        transition: transform 0.2s ease;
                    ">
                        💬
                    </div>
                    
                    <!-- Chat Window (initially hidden) -->
                    <div class="troika-chat-window" style="
                        display: none;
                        width: ${config.width};
                        height: ${config.height};
                        background: white;
                        border-radius: 12px;
                        box-shadow: 0 8px 40px rgba(0,0,0,0.2);
                        position: absolute;
                        bottom: 80px;
                        right: 0;
                        overflow: hidden;
                        border: 1px solid #e1e5e9;
                    ">
                        <!-- Header -->
                        <div class="troika-chat-header" style="
                            background: ${config.primaryColor};
                            color: white;
                            padding: 16px;
                            display: flex;
                            justify-content: space-between;
                            align-items: center;
                        ">
                            <div>
                                <h3 style="margin: 0; font-size: 16px;">Troika Assistant</h3>
                                <p style="margin: 0; font-size: 12px; opacity: 0.8;">Online</p>
                            </div>
                            <button class="troika-close-btn" style="
                                background: none;
                                border: none;
                                color: white;
                                font-size: 20px;
                                cursor: pointer;
                                padding: 4px;
                            ">×</button>
                        </div>
                        
                        <!-- Messages Area -->
                        <div class="troika-messages" style="
                            height: calc(100% - 120px);
                            overflow-y: auto;
                            padding: 16px;
                            background: #f8f9fa;
                        ">
                            <div class="troika-message bot-message" style="
                                background: white;
                                padding: 12px;
                                border-radius: 8px;
                                margin-bottom: 12px;
                                box-shadow: 0 2px 8px rgba(0,0,0,0.1);
                            ">
                                ${config.welcomeMessage}
                            </div>
                        </div>
                        
                        <!-- Input Area -->
                        <div class="troika-input-area" style="
                            padding: 16px;
                            border-top: 1px solid #e1e5e9;
                            background: white;
                        ">
                            <div style="display: flex; gap: 8px;">
                                <input type="text" class="troika-message-input" placeholder="${config.placeholder}" style="
                                    flex: 1;
                                    padding: 12px;
                                    border: 1px solid #e1e5e9;
                                    border-radius: 6px;
                                    outline: none;
                                ">
                                <button class="troika-send-btn" style="
                                    background: ${config.primaryColor};
                                    color: white;
                                    border: none;
                                    padding: 12px 16px;
                                    border-radius: 6px;
                                    cursor: pointer;
                                ">Send</button>
                            </div>
                        </div>
                    </div>
                </div>
            `;
        },
        
        getPositionStyles: function(position) {
            var styles = 'position: fixed; z-index: 9999;';
            
            switch(position) {
                case 'bottom-right':
                    return styles + 'bottom: 20px; right: 20px;';
                case 'bottom-left':
                    return styles + 'bottom: 20px; left: 20px;';
                case 'top-right':
                    return styles + 'top: 20px; right: 20px;';
                case 'top-left':
                    return styles + 'top: 20px; left: 20px;';
                default:
                    return styles + 'bottom: 20px; right: 20px;';
            }
        },
        
        addStyles: function(config) {
            if (document.getElementById('troika-widget-styles')) return;
            
            var styles = document.createElement('style');
            styles.id = 'troika-widget-styles';
            styles.textContent = `
                .troika-chat-button:hover {
                    transform: scale(1.1);
                }
                .troika-message-input:focus {
                    border-color: ${config.primaryColor};
                }
                .troika-send-btn:hover {
                    opacity: 0.9;
                }
            `;
            document.head.appendChild(styles);
        },
        
        addEventListeners: function(config) {
            var container = document.getElementById('troika-widget-' + config.projectId);
            var chatButton = container.querySelector('.troika-chat-button');
            var chatWindow = container.querySelector('.troika-chat-window');
            var closeBtn = container.querySelector('.troika-close-btn');
            var sendBtn = container.querySelector('.troika-send-btn');
            var messageInput = container.querySelector('.troika-message-input');
            
            // Toggle chat window
            chatButton.onclick = function() {
                chatWindow.style.display = chatWindow.style.display === 'none' ? 'block' : 'none';
            };
            
            // Close chat window
            closeBtn.onclick = function() {
                chatWindow.style.display = 'none';
            };
            
            // Send message
            var sendMessage = function() {
                var message = messageInput.value.trim();
                if (message) {
                    console.log('Sending message:', message);
                    // Add user message to chat
                    // Here you would typically send to your backend API
                    messageInput.value = '';
                }
            };
            
            sendBtn.onclick = sendMessage;
            messageInput.onkeypress = function(e) {
                if (e.key === 'Enter') {
                    sendMessage();
                }
            };
        }
    };
    
    // Auto-initialize if data-project-id is present
    var scripts = document.querySelectorAll('script[data-project-id]');
    scripts.forEach(function(script) {
        var projectId = script.getAttribute('data-project-id');
        if (projectId) {
            window.TroikaChatbot.init({
                projectId: projectId
            });
        }
    });
})();
