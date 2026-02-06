import React, { PropsWithChildren } from 'react';
import {
    Sidebar,
    SidebarDivider,
    SidebarGroup,
    SidebarItem,
    SidebarPage,
    SidebarSpace,
} from '@backstage/core-components';
import {
    HomeIcon,
    CategoryIcon,
    SettingsIcon,
} from '@material-ui/icons';
import { LogoFull, LogoIcon } from './Logo';
import { SidebarLogo } from './SidebarLogo';

export const Root = ({ children }: PropsWithChildren<{}>) => (
    <SidebarPage>
        <Sidebar>
            <SidebarLogo />
            <SidebarDivider />
            <SidebarGroup label="Menu" icon={<CategoryIcon />}>
                <SidebarItem icon={HomeIcon} to="/" text="Home" />
                <SidebarItem icon={CategoryIcon} to="catalog" text="Catalog" />
            </SidebarGroup>
            <SidebarSpace />
            <SidebarDivider />
            <SidebarGroup label="Settings" icon={<SettingsIcon />}>
                <SidebarItem icon={SettingsIcon} to="settings" text="Settings" />
            </SidebarGroup>
        </Sidebar>
        {children}
    </SidebarPage>
);
