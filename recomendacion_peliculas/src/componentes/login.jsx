import { useState } from "react";

export default function Login({ onLogin }) {
  const [userId, setUserId] = useState("");

  const handleSubmit = (e) => {
    e.preventDefault();

    const id = parseInt(userId);

    if (isNaN(id) || id <= 0 || id > 200000) {
      alert("El ID ingresado no es válido.");
      return;
    }

    onLogin(id);
  };

  return (
    <div className="login-page">
      <div className="login-box">
        <h2>Ingrese su usuario</h2>

        <form onSubmit={handleSubmit}>
          <input
            type="number"
            placeholder="User ID"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
          />

          <button type="submit">Ingresar</button>
        </form>
      </div>
    </div>
  );
}