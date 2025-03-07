import NavBar from "@/components/NavBar";
import UsersTable from "@/components/UsersComponent";

interface User {
  userId: string;
  name: string;
  image?: string;
  email: string;
  role: string;
  createdAt?: string;
  updatedAt?: string;
}

// const API_URL =
//   typeof window === "undefined" ? "http://app-server:5000" : "http://localhost:5000";

const API_URL = "http://localhost:5000";

const getUsers = async (): Promise<User[]> => {
  try {
    const response = await fetch(`${API_URL}/users`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
      cache: "no-store",
    });

    if (!response.ok) {
      throw new Error("Failed to fetch videos");
    }

    const users: User[] = await response.json();
    return users;
  } catch {
    // console.error("Error fetching users:", error);
    return []; // Return an empty array on error
  }
};

export const dynamic = "force-dynamic";

export default function DashboardPage() {
  const usersPromise = getUsers();
  return (
    <>
      <NavBar />
      <main className="mx-auto w-full max-w-7xl px-4 md:px-20  mt-[14vh]">
        <UsersTable userPromise={usersPromise} />
      </main>
    </>
  );
}
