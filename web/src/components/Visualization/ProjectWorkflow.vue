<template>
  <div class="h-full w-full bg-gray-50">
    <VueFlow
      v-model:nodes="nodes"
      v-model:edges="edges"
      :fit-view-on-init="true"
      :nodes-draggable="false"
      :edges-updatable="false"
      :prevent-scrolling="false"
      :auto-connect="false"
      :elevate-edges-on-select="false"
      @node-click="onNodeClick"
      @node-context-menu="onNodeContextMenu"
    >
      <template #node-custom="nodeProps">
        <div @click="() => handleNodeClick(nodeProps)" @contextmenu.prevent="(event) => handleNodeContextMenu(event, nodeProps)">
          <CustomNode 
            :node-type="nodeProps.data.nodeType" 
            :node-name="nodeProps.data.nodeName"
            :messages="nodeProps.data.messages || 0"
            :has-message-data="nodeProps.data.hasMessageData || false"
            class="cursor-pointer hover:shadow-md transition-shadow duration-200"
          />
        </div>
      </template>

      <Background :pattern-color="'#e5e7eb'" :gap="10" />
      <Controls :position="'top-right'" />
    </VueFlow>

    <!-- Right-click menu -->
    <div v-if="showContextMenu" class="context-menu" :style="contextMenuStyle">
      <div class="bg-white rounded-lg shadow-lg border border-gray-200 py-1 min-w-[160px]">
        <button 
          class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 flex items-center"
          @click="viewSampleData"
        >
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
          </svg>
          View Sample Data
        </button>
      </div>
    </div>

    <!-- Sample data modal -->
    <div v-if="showSampleModal" class="fixed inset-0 bg-black bg-opacity-30 flex items-center justify-center z-50">
      <div class="bg-white rounded-lg shadow-xl w-3/4 max-w-4xl">
        <div class="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
          <h3 class="text-lg font-medium">Sample Data - {{ selectedNode?.data.nodeType }} ({{ selectedNode?.data.nodeName }})</h3>
          <button @click="closeSampleModal" class="text-gray-400 hover:text-gray-500">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div class="p-6 max-h-[70vh] overflow-auto">
          <div v-if="loadingSamples" class="flex justify-center items-center py-8">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
          </div>
          <div v-else-if="!sampleDataGrouped || Object.keys(sampleDataGrouped).length === 0" class="text-center text-gray-500 py-8">
            No sample data available
          </div>
          <div v-else>
            <!-- Simplified structure with less nesting -->
            <div v-for="(samples, projectNodeSequence) in sampleDataGrouped" :key="projectNodeSequence" class="mb-6">
              <div class="mb-2 flex items-center justify-between">
                <h4 class="text-sm font-medium text-gray-700">Project Node Sequence: {{ projectNodeSequence }}</h4>
                <span class="text-xs bg-blue-100 text-blue-800 px-2 py-1 rounded-full">{{ samples.length }} samples</span>
              </div>
              
              <div v-for="(sample, index) in samples.slice(0, 5)" :key="index" class="mb-3">
                <div class="text-xs text-gray-500 mb-1 flex justify-between">
                  <span>Sample {{ index + 1 }}</span>
                  <span v-if="sample.timestamp">{{ new Date(sample.timestamp).toLocaleString('en-US', {
                    year: 'numeric',
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                    hour12: false
                  }) }}</span>
                </div>
                <JsonViewer :value="sample.data || sample" height="auto" />
              </div>
              
              <div v-if="samples.length > 5" class="text-center text-xs text-gray-500 mb-4">
                ... and {{ samples.length - 5 }} more samples
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted, computed } from 'vue';
import { VueFlow } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import { useRouter } from 'vue-router';
import dagre from 'dagre';
import yaml from 'js-yaml';
import CustomNode from './CustomNode.vue';
import JsonViewer from '../JsonViewer.vue';
import { hubApi } from '../../api';

const router = useRouter();

const props = defineProps({
    projectContent: {
      type: String,
      required: true,
    },
    projectId: {
      type: String,
      required: false,
    },
    enableMessages: {
      type: Boolean,
      default: true,
    },
});

const nodes = ref([]);
const edges = ref([]);

// Message Data related  
const messageData = ref({});
const messageLoading = ref(false);
const messageRefreshInterval = ref(null);

// Component sequences data
const componentSequences = ref({});

// Right-click menu related
const showContextMenu = ref(false);
const contextMenuStyle = ref({
  position: 'fixed',
  top: '0px',
  left: '0px',
});
const selectedNode = ref(null);

// Sample data related
const showSampleModal = ref(false);
const loadingSamples = ref(false);
const sampleDataRaw = ref({});

// Computed property to group sample data by ProjectNodeSequence
const sampleDataGrouped = computed(() => {
  return sampleDataRaw.value;
});

// VueFlow node click handler (keeping compatibility)
function onNodeClick(event, node) {
  handleNodeClick(node);
}

// VueFlow context menu handler (keeping compatibility)
function onNodeContextMenu(event, node) {
  handleNodeContextMenu(event, node);
}

// New node click handler
function handleNodeClick(nodeProps) {
  if (!nodeProps || !nodeProps.data) {
    console.warn('Invalid nodeProps:', nodeProps);
    return;
  }
  
  const type = nodeProps.data.nodeType?.toLowerCase();
  const id = nodeProps.data.componentId;
  
  if (!type || !id) {
    console.warn('Invalid node data:', nodeProps.data);
    return;
  }

  // Determine route based on node type
  let routePath;
  switch (type) {
    case 'input':
      routePath = `/app/inputs/${id}`;
      break;
    case 'output':
      routePath = `/app/outputs/${id}`;
      break;
    case 'ruleset':
      routePath = `/app/rulesets/${id}`;
      break;
    default:
      console.warn('Unsupported node type:', type);
      return;
  }

  // Open component details page in new tab
  const url = window.location.origin + routePath;
  window.open(url, '_blank');
}

// New context menu handler
function handleNodeContextMenu(event, nodeProps) {
  event.preventDefault();
  event.stopPropagation();
  showContextMenu.value = true;
  contextMenuStyle.value = {
    position: 'fixed',
    top: `${event.clientY}px`,
    left: `${event.clientX}px`,
  };
  selectedNode.value = nodeProps;
}

// Listen for global click events to close context menu
function onGlobalClick(event) {
  if (event.target.closest('.context-menu')) return;
  showContextMenu.value = false;
}

// Handle ESC key press
function handleEscKey(event) {
  if (event.key === 'Escape') {
    if (showSampleModal.value) {
      closeSampleModal();
    } else if (showContextMenu.value) {
      showContextMenu.value = false;
    }
  }
}

// Add global click event listener on component mount
onMounted(() => {
  document.addEventListener('click', onGlobalClick);
  document.addEventListener('keydown', handleEscKey);
  
  // Listen for project operation events to refresh message data
  const handleProjectOperation = (event) => {
    const { operation, projectId } = event.detail || {};
    if (props.projectId === projectId && (operation === 'restart' || operation === 'start' || operation === 'stop')) {
      console.log(`[ProjectWorkflow] Project operation detected: ${operation} for ${projectId}`);
      
      // Immediate refresh to clear old data
      if (props.enableMessages && props.projectId) {
        fetchMessageData();
      }
      
      // Additional delayed refresh to ensure backend updates are captured
      setTimeout(() => {
        if (props.enableMessages && props.projectId) {
          console.log(`[ProjectWorkflow] Delayed refresh after ${operation}`);
          fetchMessageData();
        }
      }, 3000); // Increased delay to ensure backend processing completes
    }
  };
  
  window.addEventListener('projectOperation', handleProjectOperation);
  
  // Listen for cache clear events to refresh data immediately
  const handleCacheCleared = (event) => {
    const { reason } = event.detail || {};
    console.log(`[ProjectWorkflow] Cache cleared: ${reason}, refreshing data`);
    
    if (props.enableMessages && props.projectId) {
      fetchMessageData();
    }
  };
  
  window.addEventListener('cacheCleared', handleCacheCleared);
  
  // Start message data refresh if enabled and projectId is provided
  if (props.enableMessages && props.projectId) {
    startMessageRefresh();
  }
  
  // Store the handlers for cleanup
  window._projectWorkflowOperationHandler = handleProjectOperation;
  window._projectWorkflowCacheHandler = handleCacheCleared;
});

// Remove global click event listener on component unmount
onUnmounted(() => {
  document.removeEventListener('click', onGlobalClick);
  document.removeEventListener('keydown', handleEscKey);
  
  // Remove project operation event listener
  if (window._projectWorkflowOperationHandler) {
    window.removeEventListener('projectOperation', window._projectWorkflowOperationHandler);
    delete window._projectWorkflowOperationHandler;
  }
  
  // Remove cache clear event listener
  if (window._projectWorkflowCacheHandler) {
    window.removeEventListener('cacheCleared', window._projectWorkflowCacheHandler);
    delete window._projectWorkflowCacheHandler;
  }
  
  // Stop message data refresh
  stopMessageRefresh();
});

// View sample data
async function viewSampleData() {
  showContextMenu.value = false;
  showSampleModal.value = true;
  loadingSamples.value = true;
  
  try {
    const nodeType = selectedNode.value.data.nodeType.toLowerCase();
    const componentId = selectedNode.value.data.componentId;
    const projectNodeSequences = selectedNode.value.data.projectNodeSequences || [];
    
    // If we have project node sequences for this component in the current project, use them
    // Otherwise fall back to the simple construction (for components not yet processed with message data)
    let allSampleData = {};
    
    if (projectNodeSequences.length > 0) {
      // Use the actual project node sequences for this component in the current project
      for (const projectNodeSequence of projectNodeSequences) {
        try {
          const response = await hubApi.getSamplerData(nodeType, projectNodeSequence);
          
          if (response && response[nodeType]) {
            // Filter and merge sample data from all project node sequences
            Object.keys(response[nodeType]).forEach(seqKey => {
              // ProjectNodeSequence format: "INPUT.api_sec.RULESET.test" or "RULESET.test"
              // Split the sequence directly by '.'
              const sequenceComponents = seqKey.split('.')
              
              // Check if this sequence contains our target component
              // Sequence format: component1.id1.component2.id2.component3.id3...
              // We need to check if the sequence contains our component type and id
              if (sequenceComponents.length >= 2 && sequenceComponents.length % 2 === 0) {
                // Look for our component type and id in the sequence
                for (let i = 0; i < sequenceComponents.length - 1; i += 2) {
                  const currentComponentType = sequenceComponents[i].toLowerCase()
                  const currentComponentId = sequenceComponents[i + 1]
                  
                  // Check if this component matches our target
                  if (currentComponentType === nodeType && currentComponentId === componentId) {
                    allSampleData[seqKey] = response[nodeType][seqKey]
                    break
                  }
                }
              }
            })
          }
        } catch (error) {
          console.warn(`Failed to fetch sample data for ${projectNodeSequence}:`, error);
        }
      }
    } else {
      // Fallback to simple construction if project node sequences are not available
      // This might happen if messageData hasn't been loaded yet
      // Ensure we don't duplicate the component type prefix
      const projectNodeSequence = componentId.startsWith(`${nodeType}.`) ? componentId : `${nodeType}.${componentId}`;
      const response = await hubApi.getSamplerData(nodeType, projectNodeSequence)
      
      if (response && response[nodeType]) {
        // Filter the sample data to only show sequences that belong to this component
        const filteredData = {}
        
        Object.keys(response[nodeType]).forEach(projectNodeSequence => {
          // ProjectNodeSequence format: "INPUT.api_sec.RULESET.test" or "RULESET.test"
          // Split the sequence directly by '.'
          const sequenceComponents = projectNodeSequence.split('.')
          
          // Check if this sequence contains our target component
          // Sequence format: component1.id1.component2.id2.component3.id3...
          // We need to check if the sequence contains our component type and id
          if (sequenceComponents.length >= 2 && sequenceComponents.length % 2 === 0) {
            // Look for our component type and id in the sequence
            for (let i = 0; i < sequenceComponents.length - 1; i += 2) {
              const currentComponentType = sequenceComponents[i].toLowerCase()
              const currentComponentId = sequenceComponents[i + 1]
              
              // Check if this component matches our target
              if (currentComponentType === nodeType && currentComponentId === componentId) {
                filteredData[projectNodeSequence] = response[nodeType][projectNodeSequence]
                break
              }
            }
          }
        })
        
        allSampleData = filteredData
      }
    }
    
    // Store the grouped sample data
    sampleDataRaw.value = allSampleData;
  } catch (error) {
    console.error('Failed to fetch sample data:', error);
    sampleDataRaw.value = {};
  } finally {
    loadingSamples.value = false;
  }
}

// Close sample data modal
function closeSampleModal() {
  showSampleModal.value = false;
  sampleDataRaw.value = {};
}

const parseAndLayoutWorkflow = (rawProjectContent) => {
  if (!rawProjectContent) {
    nodes.value = [];
    edges.value = [];
    return;
  }

  try {
    const doc = yaml.load(rawProjectContent);
    const content = doc.content || '';
    const lines = content.trim().split('\n');
    
    const tempNodes = new Map();
    const tempEdges = [];

    lines.forEach((line, index) => {
      if (!line.trim() || !line.includes('->')) return;
      const parts = line.split('->');
      if (parts.length !== 2) return;
      
      const fromId = parts[0].trim();
      const toId = parts[1].trim();
      
      const addNode = (id) => {
        if (id && !tempNodes.has(id)) {
          const [type, ...nameParts] = id.split('.');
          const name = nameParts.join('.') || type;
          tempNodes.set(id, {
            id: id,
            type: 'custom',
            data: { 
              nodeType: type.toUpperCase(), 
              nodeName: name,
              componentId: name,
              originalId: id,
              projectNodeSequences: [] // Initialize empty array, will be populated by updateNodesWithMessages
            }
          });
        }
      };

      addNode(fromId);
      addNode(toId);
      
      tempEdges.push({ 
        id: `e-${fromId}-${toId}-${index}`, 
        source: fromId, 
        target: toId,
        type: 'default',
        style: { stroke: '#9ca3af', strokeWidth: 1.2 },
        markerEnd: { type: 'arrowclosed', color: '#9ca3af' }
      });
    });

    const newNodes = Array.from(tempNodes.values());
    
    const g = new dagre.graphlib.Graph();
    g.setDefaultEdgeLabel(() => ({}));
    g.setGraph({ rankdir: 'TB', nodesep: 80, ranksep: 100 });

    newNodes.forEach(node => {
      g.setNode(node.id, { width: 75, height: 38 });
    });
    tempEdges.forEach(edge => {
      g.setEdge(edge.source, edge.target);
    });
    
    dagre.layout(g);

    nodes.value = newNodes.map(node => {
      const nodeWithPosition = g.node(node.id);
      return {
        ...node,
        position: { x: nodeWithPosition.x - 37.5, y: nodeWithPosition.y - 19 },
      };
    });

    edges.value = tempEdges;

    // Update nodes with message data if available, or set basic project node sequences
    if (props.enableMessages && props.projectId && Object.keys(messageData.value).length > 0) {
      updateNodesWithMessages();
    } else {
      // Set basic project node sequences for components even without message data
      setBasicProjectNodeSequences();
    }

  } catch (e) {
    console.error('Error parsing workflow:', e);
    nodes.value = [];
    edges.value = [];
  }
};

watch(() => props.projectContent, (newVal) => {
  parseAndLayoutWorkflow(newVal);
}, { immediate: true, deep: true });

// Watch for projectId changes
watch(() => props.projectId, (newVal, oldVal) => {
  if (newVal !== oldVal) {
    // Stop old refresh interval
    stopMessageRefresh();
    
    // Start new refresh if enabled and projectId is provided
    if (props.enableMessages && newVal) {
      startMessageRefresh();
    }
  }
}, { immediate: false });

// Watch for enableMessages changes
watch(() => props.enableMessages, (newVal) => {
  if (newVal && props.projectId) {
    startMessageRefresh();
  } else {
    stopMessageRefresh();
  }
});

// Set basic project node sequences for components when backend data is not available
function setBasicProjectNodeSequences() {
  nodes.value = nodes.value.map(node => {
    const componentType = node.data.nodeType.toLowerCase();
    const componentId = node.data.componentId;
    
    // Try to get sequences from backend data first
    let projectNodeSequences = [];
    if (componentSequences.value && componentSequences.value[componentType] && componentSequences.value[componentType][componentId]) {
      projectNodeSequences = componentSequences.value[componentType][componentId];
    } else {
      // Fallback to basic sequence only if backend data is not available
      projectNodeSequences = [`${componentType.toUpperCase()}.${componentId}`];
    }
    
    return {
      ...node,
      data: {
        ...node.data,
        messages: 0,
        hasMessageData: false,
        projectNodeSequences: projectNodeSequences
      }
    };
  });
}

// Update nodes with message information using backend-provided component sequences
function updateNodesWithMessages() {
  nodes.value = nodes.value.map(node => {
    const componentType = node.data.nodeType.toLowerCase();
    const componentId = node.data.componentId;
    
    // Get project node sequences from backend data
    let projectNodeSequences = [];
    if (componentSequences.value && componentSequences.value[componentType] && componentSequences.value[componentType][componentId]) {
      projectNodeSequences = componentSequences.value[componentType][componentId];
    } else {
      // Fallback to basic sequence if backend data is not available
      projectNodeSequences = [`${componentType.toUpperCase()}.${componentId}`];
    }
    
    // Calculate total messages using the project node sequences from backend
    let totalMessages = 0;
    // Check both data field and root level for compatibility
    const sourceData = messageData.value.data || messageData.value;
    for (const sequence of projectNodeSequences) {
      if (sourceData[sequence] && typeof sourceData[sequence] === 'object') {
        // Handle both uppercase and lowercase formats from backend
        totalMessages += sourceData[sequence].daily_messages || sourceData[sequence].DAILY_MESSAGES || 0;
      }
    }
    
    // For running projects, always show message data (even if 0)
    // This ensures that all components in a running project display MSG/D
    const isRunningProject = props.projectId && props.enableMessages;
    
    return {
      ...node,
      data: {
        ...node.data,
        messages: totalMessages, // Real message count for today (could be 0)
        hasMessageData: isRunningProject, // Show MSG/D for all components in running projects
        projectNodeSequences: projectNodeSequences // Store the actual project node sequences from backend
      }
    };
  });
}

// Fetch message data and component sequences for the project
async function fetchMessageData() {
  if (!props.projectId || !props.enableMessages) {
    // If not enabled, ensure all nodes have hasMessageData = false
    nodes.value = nodes.value.map(node => ({
      ...node,
      data: {
        ...node.data,
        messages: 0,
        hasMessageData: false,
        projectNodeSequences: []
      }
    }));
    return;
  }
  
  try {
    messageLoading.value = true;
    
    // Fetch both message data and component sequences in parallel
    // Add timestamp to break any HTTP caching
    const timestamp = Date.now();
    const [messageResponse, sequenceResponse] = await Promise.all([
      hubApi.getProjectDailyMessages(props.projectId, { _t: timestamp }),
      hubApi.getProjectComponentSequences(props.projectId, { _t: timestamp })
    ]);
    
    messageData.value = messageResponse || {};
    componentSequences.value = sequenceResponse.data || {};
    
    // Debug: Log message data for troubleshooting (only on first load or errors)
    if (process.env.NODE_ENV === 'development' && !messageData.value.data) {
      console.log(`[ProjectWorkflow] Initial message data for project ${props.projectId}:`, messageResponse);
      console.log(`[ProjectWorkflow] Initial component sequences:`, sequenceResponse.data);
    }
    
    // Update nodes with message data (including 0 values)
    updateNodesWithMessages();
  } catch (error) {
    console.error('Failed to fetch project data:', error);
    messageData.value = {};
    componentSequences.value = {};
    // Still update nodes to show 0 messages for running projects
    updateNodesWithMessages();
  } finally {
    messageLoading.value = false;
  }
}

// Start message data refresh interval
function startMessageRefresh() {
  // Initial fetch
  fetchMessageData();
  
  // Set up interval for periodic refresh (every 5 seconds for faster updates)
  messageRefreshInterval.value = setInterval(() => {
    fetchMessageData();
  }, 5000);
}

// Stop message data refresh interval
function stopMessageRefresh() {
  if (messageRefreshInterval.value) {
    clearInterval(messageRefreshInterval.value);
    messageRefreshInterval.value = null;
  }
}
</script> 

<style>
@import '@vue-flow/core/dist/style.css';
@import '@vue-flow/controls/dist/style.css';

.vue-flow__attribution {
    display: none;
}

.vue-flow__node {
  border: none !important;
  box-shadow: none !important;
  background-color: transparent !important;
  transition: transform 0.2s ease;
}

.vue-flow__node:hover {
  transform: scale(1.02);
}

.context-menu {
  z-index: 1000;
}

/* 限制控制按钮在预览区域内 */
.vue-flow__controls {
  position: absolute !important;
  top: 10px !important;
  right: 10px !important;
  left: auto !important;
  max-width: calc(100% - 20px) !important;
  z-index: 100 !important;
}

/* 确保控制按钮不会溢出到右侧 */
.vue-flow__controls .vue-flow__controls-button {
  display: inline-block !important;
  margin-right: 5px !important;
}
</style> 