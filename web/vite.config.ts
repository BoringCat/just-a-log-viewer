import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import ElementPlus from 'unplugin-element-plus/vite'

const libraryChunkMap:{[key:string]:string} = {
  "element-plus": "element-plus",
  vue: "vue",
};

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
            const matchedLibrary = Object.keys(libraryChunkMap).find((library) =>
              id.includes(library)
            );
            // 如果找到了匹配的库名，返回对应的chunk名称（从libraryChunkMap中获取）
            if (matchedLibrary) {
              return libraryChunkMap[matchedLibrary];
            } else {
              // 如果未找到匹配的库名，将该第三方依赖归入默认的'vendor' chunk
              return "vendor";
            }
          } else {
            // 如果模块ID不包含'node_modules'，即不是第三方依赖，则将其归入'index' chunk
            return "index";
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
