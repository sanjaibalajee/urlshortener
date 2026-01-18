import { redirect } from 'next/navigation'
import { getRedirectUrl } from '@/lib/api'

interface PageProps {
  params: Promise<{ shortCode: string }>
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