import React from 'react'
import { NavigationContainer } from '@react-navigation/native'
import { createNativeStackNavigator } from '@react-navigation/native-stack'
import { Text } from 'react-native'

const Stack = createNativeStackNavigator()

function PlaceholderScreen(): JSX.Element {
  return <Text>Barber SaaS MVP scaffold</Text>
}

export function RootNavigator(): JSX.Element {
  return (
    <NavigationContainer>
      <Stack.Navigator screenOptions={{ headerShown: false }}>
        <Stack.Screen name="Root" component={PlaceholderScreen} />
      </Stack.Navigator>
    </NavigationContainer>
  )
}
