export default function DashboardHome() {
  const stats = [
    { label: 'Total Sermons', value: '847', change: '+12 this month', icon: '🎵' },
    { label: 'Total Views', value: '15.2K', change: '+8% from last month', icon: '👁️' },
    { label: 'Active Members', value: '312', change: '+5 new this week', icon: '👥' },
    { label: 'Upcoming Events', value: '8', change: '3 this week', icon: '📅' },
  ]

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-600 mt-2">Welcome back! Here's what's happening at WPGC.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {stats.map((stat) => (
          <div key={stat.label} className="bg-white rounded-lg shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <span className="text-3xl">{stat.icon}</span>
              <span className="text-xs text-green-600 font-medium">{stat.change}</span>
            </div>
            <h3 className="text-2xl font-bold text-gray-900">{stat.value}</h3>
            <p className="text-sm text-gray-600 mt-1">{stat.label}</p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-lg shadow-sm p-6">
          <h2 className="text-xl font-semibold mb-4">Recent Sermons</h2>
          <div className="space-y-3">
            {[
              { title: 'The Power of Faith', speaker: 'Pastor John', date: '2025-09-01' },
              { title: 'Walking in Love', speaker: 'Elder Mark', date: '2025-08-25' },
              { title: 'Grace and Truth', speaker: 'Pastor John', date: '2025-08-18' },
            ].map((sermon) => (
              <div key={sermon.title} className="flex items-center justify-between py-3 border-b last:border-0">
                <div>
                  <p className="font-medium text-gray-900">{sermon.title}</p>
                  <p className="text-sm text-gray-500">{sermon.speaker}</p>
                </div>
                <span className="text-sm text-gray-400">{sermon.date}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="bg-white rounded-lg shadow-sm p-6">
          <h2 className="text-xl font-semibold mb-4">Quick Actions</h2>
          <div className="grid grid-cols-2 gap-4">
            <a href="/sermons/upload" className="p-4 bg-purple-50 rounded-lg hover:bg-purple-100 transition-colors">
              <span className="text-2xl mb-2 block">🎵</span>
              <p className="font-medium text-purple-900">Upload Sermon</p>
            </a>
            <a href="/events/new" className="p-4 bg-blue-50 rounded-lg hover:bg-blue-100 transition-colors">
              <span className="text-2xl mb-2 block">📅</span>
              <p className="font-medium text-blue-900">Create Event</p>
            </a>
            <a href="/members" className="p-4 bg-green-50 rounded-lg hover:bg-green-100 transition-colors">
              <span className="text-2xl mb-2 block">👥</span>
              <p className="font-medium text-green-900">Manage Members</p>
            </a>
            <a href="/media" className="p-4 bg-orange-50 rounded-lg hover:bg-orange-100 transition-colors">
              <span className="text-2xl mb-2 block">🎬</span>
              <p className="font-medium text-orange-900">Media Library</p>
            </a>
          </div>
        </div>
      </div>
    </div>
  )
}