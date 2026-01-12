import defaultRoute from './defaultRoute'

export default (api) => [
  {
    path: '/',
    redirect: () => defaultRoute(api)
  },
  {
    path: '/auth/login',
    component: () => import('@/views/Login.vue'),
    name: 'Login',
    meta: {
      noAuth: true
    }
  },
  {
    path: '/auth/register',
    component: () => import('@/views/Registration.vue'),
    name: 'Register',
    meta: {
      noAuth: true
    }
  },
  {
    path: '/auth/invite',
    component: () => import('@/views/Invite.vue'),
    name: 'Invite',
    meta: {
      noAuth: true
    }
  },
  {
    path: '/servers',
    component: () => import('@/views/ServerList.vue'),
    name: 'ServerList',
    meta: {
      tkey: 'servers.Servers',
      permission: true,
      icon: 'server',
      hotkey: 'g s'
    }
  },
  {
    path: '/servers/new',
    component: () => import('@/views/ServerCreate.vue'),
    name: 'ServerCreate'
  },
  {
    path: '/servers/view/:id',
    component: () => import('@/views/ServerView.vue'),
    name: 'ServerView'
  },
  {
    path: '/nodes',
    component: () => import('@/views/NodeList.vue'),
    name: 'NodeList',
    meta: {
      tkey: 'nodes.Nodes',
      permission: 'nodes.view',
      icon: 'node',
      hotkey: 'g n'
    }
  },
  {
    path: '/nodes/new',
    component: () => import('@/views/NodeCreate.vue'),
    name: 'NodeCreate'
  },
  {
    path: '/nodes/view/:id',
    component: () => import('@/views/NodeView.vue'),
    name: 'NodeView'
  },
  {
    path: '/users',
    component: () => import('@/views/UserList.vue'),
    name: 'UserList',
    meta: {
      tkey: 'users.Users',
      permission: 'users.info.view',
      icon: 'users',
      hotkey: 'g u'
    }
  },
  {
    path: '/users/new',
    component: () => import('@/views/UserCreate.vue'),
    name: 'UserCreate'
  },
  {
    path: '/users/view/:id',
    component: () => import('@/views/UserView.vue'),
    name: 'UserView'
  },
  {
    path: '/templates',
    component: () => import('@/views/TemplateList.vue'),
    name: 'TemplateList',
    meta: {
      tkey: 'templates.Templates',
      permission: 'templates.view',
      icon: 'template',
      hotkey: 'g t'
    }
  },
  {
    path: '/templates/new',
    component: () => import('@/views/TemplateCreate.vue'),
    name: 'TemplateCreate'
  },
  {
    path: '/templates/view/:repo/:id',
    component: () => import('@/views/TemplateView.vue'),
    name: 'TemplateView'
  },
  {
    path: '/settings',
    component: () => import('@/views/Settings.vue'),
    name: 'Settings',
    meta: {
      tkey: 'settings.Settings',
      permission: true, // Cambiado para que cualquier usuario autenticado pueda acceder
      icon: 'settings',
      hotkey: 'g c'
    }
  },
  {
    path: '/self',
    component: () => import('@/views/Self.vue'),
    name: 'Self'
  },
  {
    path: '/uptime',
    component: () => import('@/views/Uptime.vue'),
    name: 'Uptime',
    meta: {
      tkey: 'uptime.Uptime',
      permission: 'uptime.view',
      icon: 'uptime',
      hotkey: 'g t'
    }
  },
  {
    path: '/admin',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: {
      permission: 'admin',
      noAuth: false
    },
    children: [
      {
        path: '',
        component: () => import('@/views/Admin.vue'),
        name: 'Admin',
        meta: {
          tkey: 'admin.Admin',
          permission: 'admin',
          icon: 'hi-shield-check',
          hotkey: 'g a'
        }
      },
      {
        path: 'settings',
        component: () => import('@/views/Settings.vue'),
        name: 'Admin.Settings',
        meta: {
          tkey: 'settings.Settings',
          permission: true,
          icon: 'hi-cog'
        }
      },
      {
        path: 'templates',
        component: () => import('@/views/TemplateList.vue'),
        name: 'Admin.TemplateList',
        meta: {
          tkey: 'templates.Templates',
          permission: 'templates.view',
          icon: 'hi-document'
        }
      },
      {
        path: 'servers',
        component: () => import('@/views/ServerList.vue'),
        name: 'Admin.ServerList',
        meta: {
          tkey: 'servers.Servers',
          permission: true,
          icon: 'hi-server'
        }
      },
      {
        path: 'servers/new',
        component: () => import('@/views/ServerCreate.vue'),
        name: 'Admin.ServerCreate'
      },
      {
        path: 'users',
        component: () => import('@/views/UserList.vue'),
        name: 'Admin.UserList',
        meta: {
          tkey: 'users.Users',
          permission: 'users.info.view',
          icon: 'hi-users'
        }
      },
      {
        path: 'users/new',
        component: () => import('@/views/UserCreate.vue'),
        name: 'Admin.UserCreate'
      },
      {
        path: 'users/view/:id',
        component: () => import('@/views/UserView.vue'),
        name: 'Admin.UserView'
      },
      {
        path: 'templates/new',
        component: () => import('@/views/TemplateCreate.vue'),
        name: 'Admin.TemplateCreate'
      },
      {
        path: 'templates/view/:repo/:id',
        component: () => import('@/views/TemplateView.vue'),
        name: 'Admin.TemplateView'
      },
      {
        path: 'nodes',
        component: () => import('@/views/NodeList.vue'),
        name: 'Admin.NodeList',
        meta: {
          tkey: 'nodes.Nodes',
          permission: 'nodes.view',
          icon: 'hi-server'
        }
      },
      {
        path: 'nodes/new',
        component: () => import('@/views/NodeCreate.vue'),
        name: 'Admin.NodeCreate'
      },
      {
        path: 'nodes/view/:id',
        component: () => import('@/views/NodeView.vue'),
        name: 'Admin.NodeView'
      },
      {
        path: 'roles',
        component: () => import('@/views/RoleList.vue'),
        name: 'Admin.RoleList',
        meta: {
          tkey: 'roles.Roles',
          permission: 'admin',
          icon: 'hi-shield'
        }
      },
      {
        path: 'roles/new',
        component: () => import('@/views/RoleView.vue'),
        name: 'Admin.RoleCreate'
      },
      {
        path: 'roles/view/:id',
        component: () => import('@/views/RoleView.vue'),
        name: 'Admin.RoleView'
      }
    ]
  }
]
