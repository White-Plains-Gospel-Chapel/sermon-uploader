'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'

const menuItems = [
  {
    title: 'Dashboard',
    href: '/',
    icon: '📊',
  },
  {
    title: 'Sermon Upload',
    href: '/sermons/upload',
    icon: '🎵',
  },
  {
    title: 'Sermon Library',
    href: '/sermons',
    icon: '📚',
  },
  {
    title: 'Media',
    href: '/media',
    icon: '🎬',
  },
  {
    title: 'Events',
    href: '/events',
    icon: '📅',
  },
  {
    title: 'Members',
    href: '/members',
    icon: '👥',
  },
  {
    title: 'Settings',
    href: '/settings',
    icon: '⚙️',
  },
]

export default function Sidebar() {
  const pathname = usePathname()

  return (
    <div className="bg-gray-900 text-white w-64 flex flex-col">
      <div className="p-6">
        <h1 className="text-2xl font-bold">WPGC Admin</h1>
        <p className="text-gray-400 text-sm mt-1">Dashboard</p>
      </div>
      
      <nav className="flex-1 px-4 pb-4">
        {menuItems.map((item) => {
          const isActive = pathname === item.href || 
                          (item.href !== '/' && pathname.startsWith(item.href))
          
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center space-x-3 px-4 py-3 rounded-lg mb-1 transition-colors ${
                isActive
                  ? 'bg-purple-600 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              }`}
            >
              <span className="text-xl">{item.icon}</span>
              <span>{item.title}</span>
            </Link>
          )
        })}
      </nav>
      
      <div className="p-4 border-t border-gray-800">
        <div className="flex items-center space-x-3">
          <div className="w-10 h-10 bg-purple-600 rounded-full flex items-center justify-center">
            <span className="text-sm font-bold">GA</span>
          </div>
          <div>
            <p className="text-sm font-medium">Gaius Admin</p>
            <p className="text-xs text-gray-400">admin@wpgc.church</p>
          </div>
        </div>
      </div>
    </div>
  )
}