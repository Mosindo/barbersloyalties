import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TamaguiProvider } from 'tamagui'

import { RootNavigator } from '../navigation/RootNavigator'
import tamaguiConfig from '../theme/tamagui.config'

const queryClient = new QueryClient()

export function App(): JSX.Element {
  return (
    <QueryClientProvider client={queryClient}>
      <TamaguiProvider config={tamaguiConfig} defaultTheme="light">
        <RootNavigator />
      </TamaguiProvider>
    </QueryClientProvider>
  )
}
