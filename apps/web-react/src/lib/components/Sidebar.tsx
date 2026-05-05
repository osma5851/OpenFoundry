import { NavLink } from 'react-router-dom';

export function Sidebar() {
  return (
    <aside className="of-sidebar">
      <div className="of-sidebar__brand">
        <NavLink to="/" className="of-sidebar__logo" aria-label="Home">
          OF
        </NavLink>
      </div>
      <nav className="of-sidebar__section">
        <div className="of-sidebar__heading">Workspace</div>
        <NavLink
          to="/"
          end
          className="of-sidebar__link"
          data-active={undefined}
        >
          <span className="of-sidebar__label">Home</span>
        </NavLink>
      </nav>
    </aside>
  );
}
