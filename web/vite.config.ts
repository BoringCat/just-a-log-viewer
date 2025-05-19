import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import ElementPlus from 'unplugin-element-plus/vite'

interface chunk {
  match: string
  name:  string
}

const libraryChunks:chunk[] = [
  {match: 'element-plus',     name: 'element-plus/vendor'},
  {match: 'vue/compiler-sfc', name: 'vue/compiler-sfc'},
  {match: 'vue',              name: 'vue/vendor'},
]

// https://vite.dev/config/
export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          // 创建一个对象映射，用于存储库名及其对应的chunk名称
          // 检查模块ID是否包含'node_modules'，即是否为第三方依赖
          if (id.includes("node_modules")) {
            // 遍历libraryChunkMap的键（即库名），查找与模块ID存在包含关系的库名
            const matchedLibrary = libraryChunks.find((library) =>
              id.includes(library.match)
            );
            // 如果找到了匹配的库名，返回对应的chunk名称（从libraryChunkMap中获取）
            if (matchedLibrary) {
              return `modules/${matchedLibrary.name}`;
            } else {
              // 如果未找到匹配的库名，将该第三方依赖归入默认的'vendor' chunk
              return "modules/vendor";
            }
          } else if (id.includes('/src/') || id.endsWith('index.html')) {
            console.log(id)
            // 如果模块ID包含'src'，即源码，则将其归入'index' chunk
            return "index";
          } else {
            // 如果未找到匹配的库名，将该第三方依赖归入默认的'vendor' chunk
            return "modules/vendor";
          }
        },
      }
    }
  },
  server: {
    proxy: {
      '/api/v1': 'http://localhost:8514'
    }
  },
  plugins: [
    vue(),
    vueDevTools(),
    ElementPlus({}),
    AutoImport({
      resolvers: [ElementPlusResolver()],
    }),
    Components({
      resolvers: [ElementPlusResolver()],
    })
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
})
