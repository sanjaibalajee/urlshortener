import { redirect } from 'next/navigation'

interface PageProps {
  params: Promise<{ shortCode: string }>
}

async function getRedirectUrl(shortCode: string): Promise<string | null> {
  try {
    const apiBaseUrl = process.env.API_BASE_URL || 'http://localhost:8080'
    const response = await fetch(`${apiBaseUrl}/${shortCode}`, {
      method: 'GET',
      redirect: 'manual',
      cache: 'no-store'
    })

    if (response.status === 302 || response.status === 301) {
      return response.headers.get('location')
    }
    
    if (response.status === 404) {
      const errorData = await response.json()
      console.log('URL not found:', errorData)
      return null
    }
    
    return null
  } catch (error) {
    console.error('Error getting redirect URL:', error)
    return null
  }
}

export default async function RedirectPage({ params }: PageProps) {
  const { shortCode } = await params
  const targetUrl = await getRedirectUrl(shortCode)

  if (targetUrl) {
    redirect(targetUrl)
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-2xl font-bold text-red-600 mb-4">URL Not Found</h1>
        <p className="text-gray-600">The short URL "{shortCode}" does not exist or has expired.</p>
      </div>
    </div>
  )
}