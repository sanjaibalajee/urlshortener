'use server'

interface ShortenUrlRequest {
  url: string
  custom_code?: string
}

interface ShortenUrlResponse {
  success: boolean
  data: {
    short_code: string
    target_url: string
    is_active: boolean
    created_at: string
  }
  message: string
}

interface ErrorResponse {
  success: false
  message: string
}

const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:8080'

export async function shortenUrl(request: ShortenUrlRequest): Promise<ShortenUrlResponse | ErrorResponse> {
  try {
    const response = await fetch(`${API_BASE_URL}/api/shorten`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data: ShortenUrlResponse = await response.json()
    return data
  } catch (error) {
    console.error('Error shortening URL:', error)
    return {
      success: false,
      message: error instanceof Error ? error.message : 'Failed to shorten URL'
    }
  }
}

export async function getRedirectUrl(shortCode: string): Promise<string | null> {
  try {
    const response = await fetch(`${API_BASE_URL}/${shortCode}`, {
      method: 'GET',
      redirect: 'manual'
    })

    if (response.status === 302 || response.status === 301) {
      return response.headers.get('location')
    }
    
    return null
  } catch (error) {
    console.error('Error getting redirect URL:', error)
    return null
  }
}