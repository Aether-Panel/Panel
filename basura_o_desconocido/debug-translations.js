// Comando mejorado para depurar traducciones en la consola del navegador
// Copia y pega todo este código en la consola (F12 → Console)

(async function debugTranslations() {
  console.log('🔍 Depurando traducciones...\n');
  
  try {
    const app = document.querySelector('#app').__vue_app__;
    if (!app) {
      console.error('❌ No se encontró la instancia de Vue');
      return;
    }
    
    const i18n = app.config.globalProperties.$i18n;
    if (!i18n) {
      console.error('❌ No se encontró i18n');
      return;
    }
    
    console.log('✅ i18n encontrado');
    console.log('📋 Idioma actual:', i18n.locale.value);
    console.log('📋 Idioma fallback:', i18n.fallbackLocale.value);
    console.log('');
    
    // Verificar estructura de mensajes
    const messages = i18n.global.messages.value[i18n.locale.value];
    if (!messages) {
      console.error('❌ No se encontraron mensajes para el idioma:', i18n.locale.value);
      console.log('📋 Idiomas disponibles:', Object.keys(i18n.global.messages.value));
      return;
    }
    
    console.log('📚 Namespaces disponibles:', Object.keys(messages).join(', '));
    console.log('');
    
    // Verificar estructura específica de nodes
    if (messages.nodes) {
      console.log('📦 Estructura de nodes:');
      console.log('   - nodes.stats existe?', !!messages.nodes.stats);
      console.log('   - nodes.system existe?', !!messages.nodes.system);
      console.log('   - nodes.features existe?', !!messages.nodes.features);
      
      if (messages.nodes.stats) {
        console.log('   - Claves en nodes.stats:', Object.keys(messages.nodes.stats));
      }
      if (messages.nodes.system) {
        console.log('   - Claves en nodes.system:', Object.keys(messages.nodes.system));
      }
      if (messages.nodes.features) {
        console.log('   - Claves en nodes.features:', Object.keys(messages.nodes.features));
      }
    } else {
      console.error('❌ messages.nodes NO EXISTE');
    }
    
    console.log('');
    console.log('🧪 Probando traducciones específicas:');
    
    // Probar diferentes formas de acceder
    const testKeys = [
      'nodes.system.Title',
      'nodes.stats.TotalServers',
      'nodes.features.version',
      'users.Role',
      'settings.LicenseSettings'
    ];
    
    testKeys.forEach(key => {
      const result = i18n.global.t(key);
      const [namespace, ...path] = key.split('.');
      
      console.log(`\n🔑 Probando: ${key}`);
      console.log(`   Namespace: ${namespace}, Path: ${path.join('.')}`);
      console.log(`   Resultado: "${result}"`);
      console.log(`   ¿Es igual a la clave? ${result === key}`);
      
      // Intentar acceso directo
      if (messages[namespace]) {
        let directAccess = messages[namespace];
        for (const part of path) {
          directAccess = directAccess?.[part];
        }
        console.log(`   Acceso directo: "${directAccess}"`);
      }
    });
    
    // Verificar si el problema es con la resolución
    console.log('\n🔧 Verificando configuración de i18n:');
    console.log('   - legacy:', i18n.global.legacy);
    console.log('   - locale:', i18n.global.locale.value);
    console.log('   - fallbackLocale:', i18n.global.fallbackLocale.value);
    
    return {
      locale: i18n.locale.value,
      messages: messages,
      nodesStructure: messages.nodes ? {
        stats: messages.nodes.stats,
        system: messages.nodes.system,
        features: messages.nodes.features
      } : null
    };
    
  } catch (error) {
    console.error('❌ Error:', error);
    console.error('Stack:', error.stack);
  }
})();

