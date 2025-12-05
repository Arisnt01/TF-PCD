import { useState, useEffect } from "react";
import Login from "./componentes/login";
import Carrusel from "./componentes/Carrusel";
import "./App.css"
//token para obtener las imagenes de la pelicula
const TMDB_ACCESS_TOKEN = "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI1MWI0YjZkZGZkNmRhOTNmZmNjZWQzOWEzM2YwNmYzMiIsIm5iZiI6MTc2NDg4NDk1MS45ODg5OTk4LCJzdWIiOiI2OTMyMDFkNzc4ODIxMzMzZmQ2OWJhOTEiLCJzY29wZXMiOlsiYXBpX3JlYWQiXSwidmVyc2lvbiI6MX0.38oLhZtmTobQyVlxAyWaF8UeKLA-Uh6vvnidDxEWVns";

function App() {
  const [userId, setUserId] = useState(null);
  const [loading, setLoading] = useState(false);

  const [userMovies, setUserMovies] = useState([]);
  const [loadingMovies, setLoadingMovies] = useState(false);

  const [recommendations, setRecommendations] = useState([]);


async function fetchPosterByMovieId(movieId) {
  try {
    const linkRes = await fetch(`http://localhost:8080/api/movie-links/${movieId}`);
    if (!linkRes.ok) return null;

    const link = await linkRes.json();
    const tmdbId = link.tmdbId;

    if (!tmdbId) return null;

    const tmdbRes = await fetch(
      `https://api.themoviedb.org/3/movie/${tmdbId}`,
      {
        headers: {
          Authorization: `Bearer ${TMDB_ACCESS_TOKEN}`,
          "Content-Type": "application/json;charset=utf-8",
        },
      }
    );

    const data = await tmdbRes.json();

    if (!data.poster_path) return null;

    return `https://image.tmdb.org/t/p/w500${data.poster_path}`;
  } catch (err) {
    console.error("Error obteniendo poster:", err);
    return null;
  }
}

  useEffect(() => {
    if (userId === null) return;

    const fetchUserMovies = async () => {
      try {
        setLoadingMovies(true);

        const res = await fetch(
          `http://localhost:8080/api/user-movies?user_id=${userId}`
        );

        if (!res.ok) throw new Error("Error al obtener películas");

        const data = await res.json();

        const watched = (data.watched || []).sort((a,b) => b.rating - a.rating)

        const moviesWithPoster = await Promise.all(
          watched.map(async (m) => ({
            ...m,
            poster: await fetchPosterByMovieId(m.movie_id),
          }))
        );

        setUserMovies(moviesWithPoster);
      } catch (error) {
        console.error(error);
        alert("Error cargando películas del usuario");
      } finally {
        setLoadingMovies(false);
      }
    };

    fetchUserMovies();
  }, [userId]);

  // Obtener recomendaciones
  const fetchRecommendations = async () => {
    try {
      setLoading(true);

      const res = await fetch("http://localhost:8080/api/recommendations", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          user_id: Number(userId),
          k: 10,
        }),
      });

      const data = await res.json();

      const recsWithPosters = await Promise.all(
        (data.recommendations || []).map(async (m) => ({
          ...m,
          poster: await fetchPosterByMovieId(m.movie_id), 
        }))
      );
      setRecommendations(recsWithPosters);
    } catch (err) {
      console.error(err);
      alert("Error obteniendo recomendaciones");
    } finally {
      setLoading(false);
    }
  };

  if (userId === null) {
    return <Login onLogin={setUserId} />;
  }

  return (
    <div className = "app-container">

      <button
        className="logout-btn"
        onClick={() => {
          setUserId(null);
          setRecommendations([]);
          setUserMovies([]);
        }}
      >
        Cerrar sesión
      </button>

      <h1>Bienvenido, Usuario {userId}</h1>

      <Carrusel 
        title="Tus películas vistas" 
        data={userMovies} 
        type="watched" 
        loading={loadingMovies}
      />

      <button 
        onClick={fetchRecommendations} 
        disabled={loading} 
        className="boton_sorpresa"
      >
        {loading ? "Cargando..." : "Descubre algo nuevo"}
      </button>

      <Carrusel 
        title="Te recomendamos" 
        data={recommendations} 
        type="recommend" 
        loading={loading}
      />

    </div>
  );
}

export default App;