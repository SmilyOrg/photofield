# Chip Component

A generic, reusable chip component following Material Design guidelines.

## Features

- **Icons**: Support for Material icons on the left side
- **Avatars**: Support for avatar text/initials (alternative to icons)
- **Removable**: Optional close button for removing chips
- **Clickable**: Interactive chips with hover states
- **Selectable**: Selected state for filter/choice chips
- **Variants**: Filled and outlined styles
- **Dense mode**: Compact sizing option
- **Disabled state**: Non-interactive state
- **Custom colors**: Support for custom color theming

## Basic Usage

```vue
<template>
  <chip text="Default Chip" />
  
  <!-- With icon -->
  <chip icon="tag" text="Tagged" />
  
  <!-- Clickable -->
  <chip 
    clickable 
    text="Click me" 
    @click="handleClick"
  />
  
  <!-- Removable -->
  <chip 
    removable 
    text="Remove me" 
    @remove="handleRemove"
  />
  
  <!-- Using slot -->
  <chip clickable>
    Custom content
  </chip>
</template>

<script setup>
import Chip from './components/chips/Chip.vue';

const handleClick = () => {
  console.log('Chip clicked!');
};

const handleRemove = () => {
  console.log('Chip removed!');
};
</script>
```

## Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `text` | String | `''` | Text content to display |
| `icon` | String | `''` | Material icon name |
| `avatar` | String | `''` | Avatar text/initials |
| `clickable` | Boolean | `false` | Enable click interactions |
| `removable` | Boolean | `false` | Show remove button |
| `selected` | Boolean | `false` | Selected state |
| `outlined` | Boolean | `false` | Use outlined variant |
| `dense` | Boolean | `false` | Use compact sizing |
| `disabled` | Boolean | `false` | Disable interactions |
| `color` | String | `''` | Custom color (CSS value) |
| `iconSize` | String/Number | `18` | Icon size |

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `click` | Event | Emitted when chip is clicked (requires `clickable`) |
| `remove` | Event | Emitted when remove button is clicked (requires `removable`) |

## Examples

### Tag Chips
```vue
<chip 
  v-for="tag in tags"
  :key="tag.id"
  icon="tag"
  :text="tag.name"
  removable
  @remove="removeTag(tag)"
/>
```

### Filter Chips
```vue
<chip 
  v-for="filter in filters"
  :key="filter.id"
  :text="filter.label"
  clickable
  :selected="filter.active"
  @click="toggleFilter(filter)"
/>
```

### Avatar Chips
```vue
<chip 
  v-for="user in users"
  :key="user.id"
  :avatar="user.initials"
  :text="user.name"
  clickable
/>
```

### Dense Chips
```vue
<chip dense icon="date_range" text="2024-01-15" />
```

### Outlined Chips
```vue
<chip outlined icon="location_on" text="San Francisco" />
```

## Styling

The component uses CSS custom properties for theming:
- `--mdc-theme-surface`: Background color
- `--mdc-theme-on-surface`: Text color
- `--mdc-theme-primary`: Selected/hover color
- `--mdc-theme-on-primary`: Text on primary color

You can override these in your app's CSS or use Material Design theme configuration.
