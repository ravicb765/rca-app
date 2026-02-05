import React from 'react';
import { render, screen } from '@testing-library/react';
import { ServiceMapComponent } from './ServiceMapComponent';

describe('ServiceMapComponent', () => {
    it('renders the graph container', () => {
        render(<ServiceMapComponent />);
        expect(screen.getByTestId('service-map-container')).toBeInTheDocument();
        expect(screen.getByText('Service Dependency Graph')).toBeInTheDocument();
    });
});
