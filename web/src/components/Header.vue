<template>
  <header class="h-8 border-b border-gray-200 flex items-center justify-between px-6">
    <!-- Empty left side -->
    <div class="flex items-center space-x-3">
    </div>
    
    <!-- Action Buttons -->
    <div class="flex items-center space-x-2">
      <button @click="openTutorial" class="px-2 py-1 text-xs bg-transparent hover:bg-gray-100 rounded flex items-center transition-colors">
        <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.746 0 3.332.477 4.5 1.253v13C19.832 18.477 18.246 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"></path>
        </svg>
        Tutorial
      </button>
      <button @click="openGitHub" class="px-2 py-1 text-xs bg-transparent hover:bg-gray-100 rounded flex items-center transition-colors">
        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
          <path fill-rule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clip-rule="evenodd"></path>
        </svg>
        GitHub
      </button>
      <button @click="logout" class="px-2 py-1 text-xs bg-transparent hover:bg-gray-100 rounded transition-colors">
        Sign out
      </button>
    </div>
  </header>
</template>

<script>
import { hubApi } from '../api';

export default {
  name: 'Header',
  methods: {
    logout() {
      hubApi.clearToken();
      this.$router.push('/');
    },
    openGitHub() {
      window.open('https://github.com/EBWi11/AgentSmith-HUB', '_blank');
    },
    openTutorial() {
      this.$router.push('/app/tutorial');
    }
  }
}
</script> 