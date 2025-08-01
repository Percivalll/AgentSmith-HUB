<template>
  <div class="tutorial-container">
    <!-- Loading State -->
    <div v-if="loading" class="loading-overlay">
      <div class="loading-spinner">
        <div class="spinner-ring"></div>
        <p class="loading-text">{{ currentLanguage === 'en' ? 'Loading tutorial content...' : '正在加载教程内容...' }}</p>
      </div>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-overlay">
      <div class="error-content">
        <div class="error-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
        </div>
        <h3 class="error-title">{{ currentLanguage === 'en' ? 'Load Failed' : '加载失败' }}</h3>
        <p class="error-message">{{ error }}</p>
        <button @click="loadTutorialContent" class="retry-button">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/>
            <path d="M21 3v5h-5"/>
            <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/>
            <path d="M3 21v-5h5"/>
          </svg>
          {{ currentLanguage === 'en' ? 'Retry' : '重新加载' }}
        </button>
      </div>
    </div>

    <!-- Main Content -->
    <div v-else class="tutorial-content">
      <!-- Content Area -->
      <div class="content-wrapper">
        <!-- Sidebar Outline -->
        <div class="outline-sidebar" :class="{ visible: showOutline }">
          <div class="outline-header">
            <h3>{{ currentLanguage === 'en' ? 'Document Outline' : '文档大纲' }}</h3>
            <button @click="toggleOutline" class="outline-close">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>
          <div class="outline-content">
            <div class="outline-list">
              <div
                v-for="item in tocItems"
                :key="item.id"
                @click="scrollToElement(item.id)"
                class="outline-item"
                :class="[
                  `level-${item.level}`,
                  { active: currentSection === item.id }
                ]"
              >
                <span class="outline-text">{{ item.text }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Document Area -->
        <div class="document-wrapper" 
             :class="{ 'outline-open': showOutline }"
             :style="{ marginLeft: showOutline ? `${outlineSidebarWidth}px` : '0' }">
          <div class="document-container" ref="documentContainer">
            <div class="markdown-body" v-html="renderedHtml"></div>
        </div>

          <!-- Status Bar -->
          <div class="status-bar">
            <div class="status-left">
              <span class="status-item">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                  <polyline points="14,2 14,8 20,8"/>
                </svg>
                {{ formatFileSize(tutorialContent.length) }}
              </span>
              <span class="status-item">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M3 3v18h18"/>
                  <rect x="7" y="7" width="3" height="9"/>
                  <rect x="13" y="5" width="3" height="11"/>
                </svg>
                {{ tocItems.length }} {{ currentLanguage === 'en' ? 'sections' : '章节' }}
              </span>
            </div>
            <div class="status-right">
              <span class="status-item">
                Markdown
              </span>
              <div class="status-controls">
                <button @click="toggleOutline" class="status-btn" :class="{ active: showOutline }" :title="currentLanguage === 'en' ? 'Document Outline' : '文档大纲'">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="8" y1="6" x2="21" y2="6"/>
                    <line x1="8" y1="12" x2="21" y2="12"/>
                    <line x1="8" y1="18" x2="21" y2="18"/>
                    <line x1="3" y1="6" x2="3.01" y2="6"/>
                    <line x1="3" y1="12" x2="3.01" y2="12"/>
                    <line x1="3" y1="18" x2="3.01" y2="18"/>
                  </svg>
                </button>
                <button @click="toggleLanguage" class="status-btn language-btn" :title="currentLanguage === 'en' ? 'Switch to Chinese' : '切换到英文'">
                  <span class="language-text">{{ currentLanguage === 'en' ? '中' : 'EN' }}</span>
                </button>
                <button @click="toggleFullscreen" class="status-btn" :title="currentLanguage === 'en' ? 'Fullscreen Mode' : '全屏模式'">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick, onBeforeUnmount, computed } from 'vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import javascript from 'highlight.js/lib/languages/javascript'
import bash from 'highlight.js/lib/languages/bash'
import 'github-markdown-css/github-markdown-light.css'
import 'highlight.js/styles/github.css'

// 注册语言
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('bash', bash)

const loading = ref(true)
const error = ref(null)
const tutorialContent = ref('')
const renderedHtml = ref('')
const tocItems = ref([])
const currentSection = ref('')
const showOutline = ref(false)
const documentContainer = ref(null)
const isFullscreen = ref(false)
const currentLanguage = ref('en') // Default to English

// 计算outline sidebar宽度（响应式）
const outlineSidebarWidth = computed(() => {
  if (typeof window !== 'undefined' && window.innerWidth <= 768) {
    return 260 // 移动端宽度
  }
  return 280 // 桌面端宽度
})

// 配置markdown-it
const md = new MarkdownIt({
  html: true,
  linkify: true,
  typographer: true,
  breaks: true, // 添加换行支持
  highlight: function (str, lang) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(str, { language: lang }).value
      } catch (err) {
        console.warn('Highlight.js error:', err)
      }
    }
    return hljs.highlightAuto(str).value
  }
})

// 确保列表项正确渲染
md.renderer.rules.list_item_open = function() {
  return '<li>';
};

md.renderer.rules.list_item_close = function() {
  return '</li>';
};

// 获取教程内容
async function loadTutorialContent() {
  try {
    loading.value = true
    error.value = null

    const fileName = currentLanguage.value === 'en' ? '/agentsmith-hub-guide.md' : '/agentsmith-hub-guide-zh.md'
    const response = await fetch(fileName)
    
    if (!response.ok) {
      throw new Error(`${currentLanguage.value === 'en' ? 'Failed to load tutorial file' : '无法加载教程文件'} (${response.status})`)
    }
    
    const markdownText = await response.text()
    
    if (!markdownText || markdownText.trim() === '') {
      throw new Error(currentLanguage.value === 'en' ? 'Tutorial file is empty' : '教程文件内容为空')
    }
    
    tutorialContent.value = markdownText
    
    // 渲染markdown
    await renderMarkdown(markdownText)
    
    // 等待DOM更新后设置滚动监听
    await nextTick()
    setupScrollListener()
    
  } catch (err) {
    console.error('Failed to load tutorial content:', err)
    error.value = err.message || (currentLanguage.value === 'en' ? 'Failed to load tutorial content' : '加载教程内容失败')
  } finally {
    loading.value = false
  }
}

// 切换语言
async function toggleLanguage() {
  const newLanguage = currentLanguage.value === 'en' ? 'zh' : 'en'
  currentLanguage.value = newLanguage
  
  // 保存语言偏好到localStorage
  localStorage.setItem('tutorial_language', newLanguage)
  
  // 重新加载内容
  await loadTutorialContent()
}

// 渲染markdown
async function renderMarkdown(markdown) {
  try {
    // 处理标题ID
    const processedMarkdown = addHeaderIds(markdown)
    
    // 渲染为HTML
    const html = md.render(processedMarkdown)
    renderedHtml.value = html

// 生成目录
    generateTableOfContents(processedMarkdown)
    
  } catch (err) {
    console.error('Markdown rendering error:', err)
    error.value = currentLanguage.value === 'en' ? 'Markdown rendering failed' : 'Markdown渲染失败'
  }
}

// 为标题添加ID
function addHeaderIds(markdown) {
  const lines = markdown.split('\n')
  const toc = []
  
  const processedLines = lines.map((line, index) => {
    const match = line.match(/^(#{1,6})\s+(.+)$/)
    if (match) {
      const level = match[1].length
      const text = match[2]
        .replace(/[🛡️🚀🧠📋🎯🔌⚡💼❓💡📖📚📊🔧🚨🔧]/g, '')
        .trim()
      
      if (text && level <= 3) {
        const id = `section-${index}`
        toc.push({
          id,
          level,
          text,
          line: index + 1
        })
        return `${match[1]} <a name="${id}"></a>${match[2]}`
      }
    }
    return line
  })
  
  tocItems.value = toc
  return processedLines.join('\n')
}

// 生成目录
function generateTableOfContents(markdown) {
  // 目录已在addHeaderIds中生成
}

// 设置滚动监听
function setupScrollListener() {
  if (!documentContainer.value) return
  
  const container = documentContainer.value
  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          const id = entry.target.getAttribute('name')
          if (id) {
            currentSection.value = id
          }
        }
      })
    },
    {
      root: container,
      rootMargin: '-10% 0px -80% 0px'
    }
  )
  
  // 观察所有标题锚点
  setTimeout(() => {
    const anchors = container.querySelectorAll('a[name]')
    anchors.forEach(anchor => observer.observe(anchor))
  }, 200)
}

// 滚动到指定元素
function scrollToElement(elementId) {
  if (!documentContainer.value) return
  
  const element = documentContainer.value.querySelector(`a[name="${elementId}"]`)
  if (element) {
    element.scrollIntoView({
      behavior: 'smooth',
      block: 'start'
    })
    currentSection.value = elementId
  }
}

// 获取章节标题
function getSectionTitle(sectionId) {
  const section = tocItems.value.find(item => item.id === sectionId)
  return section ? section.text : ''
}

// 切换大纲显示
function toggleOutline() {
  showOutline.value = !showOutline.value
}

// 切换全屏
function toggleFullscreen() {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen()
    isFullscreen.value = true
  } else {
    document.exitFullscreen()
    isFullscreen.value = false
  }
}

// 格式化文件大小
function formatFileSize(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// 组件挂载
onMounted(() => {
  // 从localStorage恢复语言偏好，默认为英文
  const savedLanguage = localStorage.getItem('tutorial_language')
  if (savedLanguage && (savedLanguage === 'en' || savedLanguage === 'zh')) {
    currentLanguage.value = savedLanguage
  }
  
  loadTutorialContent()
  
  // 监听全屏变化
  document.addEventListener('fullscreenchange', () => {
    isFullscreen.value = !!document.fullscreenElement
  })
})

// 组件卸载时清理
onBeforeUnmount(() => {
  // 清理工作已在组件挂载时处理
})
</script>

<style scoped>
.tutorial-container {
  width: 100%;
  height: 100%;
  background: #ffffff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* Loading State */
.loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.loading-spinner {
  text-align: center;
}

.spinner-ring {
  width: 40px;
  height: 40px;
  border: 3px solid #f3f4f6;
  border-top: 3px solid #3b82f6;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 16px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.loading-text {
  color: #6b7280;
  font-size: 14px;
  font-weight: 500;
  margin: 0;
}

/* Error State */
.error-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.error-content {
  text-align: center;
  max-width: 400px;
  padding: 32px;
}

.error-icon {
  color: #ef4444;
  margin-bottom: 16px;
}

.error-title {
  color: #111827;
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 8px;
}

.error-message {
  color: #6b7280;
  font-size: 14px;
  margin: 0 0 24px;
  line-height: 1.5;
}

.retry-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: #3b82f6;
  color: white;
  border: none;
  padding: 10px 16px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.retry-button:hover {
  background: #2563eb;
  transform: translateY(-1px);
}

/* Main Content */
.tutorial-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Content Wrapper */
.content-wrapper {
  flex: 1;
  display: flex;
  position: relative;
  overflow: hidden;
}

/* Outline Sidebar */
.outline-sidebar {
  position: absolute;
  top: 0;
  left: 0;
  width: 280px;
  height: 100%;
  background: white;
  border-right: 1px solid #e5e7eb;
  border-left: 1px solid #e5e7eb;
  transform: translateX(-100%);
  transition: transform 0.3s ease;
  z-index: 100;
  box-shadow: 2px 0 8px rgba(0, 0, 0, 0.1);
}

.outline-sidebar.visible {
  transform: translateX(0);
}

.outline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
}

.outline-header h3 {
  color: #111827;
  font-size: 16px;
  font-weight: 600;
  margin: 0;
}

.outline-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: none;
  border: none;
  cursor: pointer;
  color: #6b7280;
  border-radius: 4px;
  transition: all 0.2s;
}

.outline-close:hover {
  background: #e5e7eb;
  color: #374151;
}

.outline-content {
  height: calc(100% - 65px - 36px); /* Subtract header height (65px) and status bar height (36px) */
  overflow-y: auto;
  padding-bottom: 8px; /* Add some extra padding for better visual spacing */
}

.outline-list {
  padding: 12px 0;
}

.outline-item {
  display: flex;
  align-items: center;
  padding: 8px 20px;
  cursor: pointer;
  transition: all 0.2s;
  border-left: 3px solid transparent;
}

.outline-item:hover {
  background: #f3f4f6;
}

.outline-item.active {
  background: #eff6ff;
  border-left-color: #3b82f6;
  color: #1d4ed8;
}

.outline-item.level-1 {
  font-weight: 600;
  color: #111827;
  font-size: 14px;
}

.outline-item.level-2 {
  font-weight: 500;
  color: #374151;
  padding-left: 32px;
  font-size: 13px;
}

.outline-item.level-3 {
  font-weight: 400;
  color: #6b7280;
  padding-left: 44px;
  font-size: 13px;
}

.outline-text {
  flex: 1;
  line-height: 1.4;
}

/* Document Area */
.document-wrapper {
  flex: 1;
  transition: all 0.3s ease;
  display: flex;
  flex-direction: column;
}

.document-container {
  flex: 1;
  overflow-y: auto;
  background: white;
  min-height: 0;
  padding-bottom: 48px; /* leave space for fixed status bar */
}

/* Status Bar */
.status-bar {
  position: fixed;
  left: var(--sidebar-width, 288px);
  right: 0;
  bottom: 0;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  background: #f9fafb;
  border-top: 1px solid #e5e7eb;
  font-size: 12px;
  color: #6b7280;
  z-index: 500;
}

.status-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1 1 auto;
  min-width: 0;
}

.status-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  white-space: nowrap;
  flex-shrink: 0;
}

.status-item svg {
  opacity: 0.7;
  flex-shrink: 0;
}

.status-controls {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.status-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: transparent;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  color: #6b7280;
  flex-shrink: 0;
}

.status-btn:hover {
  background: #f3f4f6;
  color: #374151;
  border-color: #9ca3af;
}

.status-btn.active {
  background: #3b82f6;
  color: white;
  border-color: #3b82f6;
}

/* Language Button */
.language-btn {
  width: 28px !important;
  font-size: 11px;
  font-weight: 600;
}

.language-text {
  line-height: 1;
}

/* GitHub Markdown 样式增强 */
:deep(.markdown-body) {
  max-width: none;
  margin: 0;
  padding: 24px 40px 32px 40px;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif;
  line-height: 1.6;
  color: #24292f;
  background-color: #ffffff;
  box-sizing: border-box;
}

/* 自定义标题样式 */
:deep(.markdown-body h1) {
  border-bottom: 1px solid #d0d7de;
  padding-bottom: 0.5rem;
  margin-bottom: 2rem;
  margin-top: 0;
}

:deep(.markdown-body h2) {
  border-bottom: 1px solid #d0d7de;
  padding-bottom: 0.3rem;
  margin-bottom: 1.5rem;
  margin-top: 2rem;
}

:deep(.markdown-body h3) {
  margin-bottom: 1rem;
  margin-top: 1.5rem;
}

/* 代码块样式增强 */
:deep(.markdown-body pre) {
  background: #f6f8fa;
  border-radius: 6px;
  border: 1px solid #d0d7de;
  padding: 16px;
  overflow-x: auto;
  margin: 1.5rem 0;
}

:deep(.markdown-body code) {
  background: #f6f8fa;
  padding: 0.2em 0.4em;
  border-radius: 3px;
  font-size: 85%;
}

:deep(.markdown-body pre code) {
  background: transparent;
  padding: 0;
}

/* 表格样式 */
:deep(.markdown-body table) {
  border-collapse: collapse;
  margin: 1.5rem 0;
  width: 100%;
}

:deep(.markdown-body th) {
  background: #f6f8fa;
  font-weight: 600;
}

:deep(.markdown-body th, .markdown-body td) {
  border: 1px solid #d0d7de;
  padding: 8px 12px;
}

/* 引用块样式 */
:deep(.markdown-body blockquote) {
  border-left: 4px solid #d0d7de;
  padding: 0 1rem;
  color: #656d76;
  background: #f6f8fa;
  margin: 1.5rem 0;
}

/* 链接样式 */
:deep(.markdown-body a) {
  color: #0969da;
  text-decoration: none;
}

:deep(.markdown-body a:hover) {
  text-decoration: underline;
}

/* 列表样式 - 确保星号正确显示 */
:deep(.markdown-body ul) {
  list-style-type: disc;
  padding-left: 2em;
  margin: 1rem 0;
}

:deep(.markdown-body ol) {
  list-style-type: decimal;
  padding-left: 2em;
  margin: 1rem 0;
}

:deep(.markdown-body li) {
  margin: 0.25rem 0;
  line-height: 1.6;
}

:deep(.markdown-body ul li) {
  list-style-type: disc;
}

:deep(.markdown-body ol li) {
  list-style-type: decimal;
}

/* 嵌套列表样式 */
:deep(.markdown-body ul ul) {
  list-style-type: circle;
  margin: 0.5rem 0;
}

:deep(.markdown-body ul ul ul) {
  list-style-type: square;
  margin: 0.5rem 0;
}

:deep(.markdown-body ol ol) {
  list-style-type: lower-alpha;
  margin: 0.5rem 0;
}

:deep(.markdown-body ol ol ol) {
  list-style-type: lower-roman;
  margin: 0.5rem 0;
}

/* 自定义滚动条 */
.document-container::-webkit-scrollbar {
  width: 8px;
}

.document-container::-webkit-scrollbar-track {
  background: #f1f5f9;
}

.document-container::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

.document-container::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}

@media (max-width: 1024px) {
  :deep(.markdown-body) {
    padding: 24px 32px 32px 32px;
  }
  
  .status-controls {
    margin-left: 12px;
  }
}

@media (max-width: 768px) {
  .status-controls {
    margin-left: 8px;
  }
  
  .status-btn {
    width: 22px;
    height: 22px;
  }
  
  .language-btn {
    width: 26px !important;
  }
  
  .outline-sidebar {
    width: 260px;
  }
  
  .status-bar {
    padding: 0 16px;
  }
  
  :deep(.markdown-body) {
    padding: 20px 16px 24px 20px;
  }
}
</style> 