"use client";

import React, { use, useState } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";
import { Edit, Trash2} from "lucide-react";
import { cn } from "@/lib/utils";

interface User {
  userId: string;
  name: string;
  image?: string;
  email: string;
  role: string;
  createdAt?: string;
  updatedAt?: string;
}

interface UsersTableProps {
  userPromise: Promise<User[]>;
}

const UsersTable: React.FC<UsersTableProps> = ({ userPromise }) => {
  // Resolve the promise and set to local state.
  const resolvedUsers = use(userPromise);
  const [users, setUsers] = useState<User[]>(resolvedUsers);

  // State for search input.
  const [searchTerm, setSearchTerm] = useState<string>("");

  // State for adding a user.
  const [isAddOpen, setIsAddOpen] = useState<boolean>(false);
  const [newUser, setNewUser] = useState({
    name: "",
    email: "",
    role: "user",
    password: "",
  });

  const { toast } = useToast();

  // Filter users based on searchTerm.
  const filteredUsers = users.filter(
    (user) =>
      user.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      user.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
      user.role.toLowerCase().includes(searchTerm.toLowerCase()) ||
      user.userId.toLowerCase().includes(searchTerm.toLowerCase()),
  );

  // Add a new user.
  const handleAddUser = async () => {
    try {
      const role =
        newUser.email === "mo.kamara@integems.com" ? "superadmin" : "user";
      const res = await fetch("http://127.0.0.1:5000/signup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...newUser, role }),
      });
      if (res.ok) {
        const createdUser: User = await res.json();
        setUsers([...users, createdUser]);
        setIsAddOpen(false);
        setNewUser({ name: "", email: "", role: "user", password: "" });
        toast({
          description: (
            <p className="text-green-500">User added successfully</p>
          ),
        });
      } else {
        toast({
          description: <p className="text-red-500">Couldn&apos;t add user</p>,
        });
      }
    } catch{
      // console.error("Error adding user", err);
      toast({ description: <p className="text-red-500">Couldn&apos;t add user</p> });
    }
  };

  return (
    <div className="p-4">
      {/* Header, Search and Add Button */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 space-y-2 sm:space-y-0">
        <h2 className="text-lg text-gray-800 md:text-xl">Users</h2>
        <div className="flex items-center gap-2">
          <Input
            placeholder="Search users..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="max-w-xs"
          />
          <Button onClick={() => setIsAddOpen(true)}>Add user</Button>
        </div>
      </div>

      {/* Users Table */}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="font-black">User</TableHead>
            <TableHead className="font-black">Name</TableHead>
            <TableHead className="font-black">Email</TableHead>
            <TableHead className="font-black">Role</TableHead>
            <TableHead className="font-black">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filteredUsers.map((user) => (
            <UserTableRow user={user} key={user.userId} />
          ))}
        </TableBody>
      </Table>

      {/* Add User Modal */}
      <Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New user</DialogTitle>
          </DialogHeader>
          <div className="mt-4 space-y-4">
            <Input
              placeholder="Name"
              value={newUser.name}
              onChange={(e) => setNewUser({ ...newUser, name: e.target.value })}
            />
            <Input
              placeholder="Email"
              value={newUser.email}
              onChange={(e) =>
                setNewUser({ ...newUser, email: e.target.value })
              }
            />
            <div>
              <label className="block text-sm font-medium mb-1">Role</label>
              <Select
                value={newUser.role}
                onValueChange={(value) =>
                  setNewUser({ ...newUser, role: value })
                }
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select role" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="user">User</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Input
              type="password"
              placeholder="Password"
              value={newUser.password}
              onChange={(e) =>
                setNewUser({ ...newUser, password: e.target.value })
              }
            />
          </div>
          <DialogFooter>
            <Button onClick={handleAddUser}>Add</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default UsersTable;

const UserTableRow: React.FC<{ user: User }> = ({ user: _user }) => {
  // State for editing a user.
  const [user, setUser] = useState<User>(_user);
  const [deleted, setDeleted] = useState<boolean>(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [editRole, setEditRole] = useState<string>("");
  const [isEditOpen, setIsEditOpen] = useState<boolean>(false);
  const { toast } = useToast();

  // Delete a user.
  const handleDelete = async (userId: string) => {
    try {
      const res = await fetch(`http://127.0.0.1:5000/users/${userId}`, {
        method: "DELETE",
      });
      if (res.ok) {
        setDeleted(true);
        toast({
          description: (
            <p className="text-green-500">User deleted successfully</p>
          ),
        });
      } else {
        toast({
          description: <p className="text-red-500">Couldn&apos;t delete user</p>,
        });
      }
    } catch {
      // console.error("Error deleting user", err);
      toast({
        description: <p className="text-red-500">Couldn&apos;t delete user</p>,
      });
    }
  };

  // Open the edit modal.
  const openEditModal = (user: User) => {
    setEditUser(user);
    setEditRole(user.role);
    setIsEditOpen(true);
  };

  // Update a user.
  const handleUpdate = async () => {
    if (!editUser) return;
    const updatedUser = { ...editUser, role: editRole };
    try {
      const res = await fetch("http://127.0.0.1:5000/users", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updatedUser),
      });
      if (res.ok) {
        setUser(updatedUser);
        setIsEditOpen(false);
        toast({
          description: (
            <p className="text-green-500">User updated successfully</p>
          ),
        });
      } else {
        toast({
          description: <p className="text-red-500">Couldn&apos;t update user</p>,
        });
      }
    } catch{
      // console.error("Error updating user", err);
      toast({
        description: <p className="text-red-500">Couldn&apos;t update user</p>,
      });
    }
  };

  if (deleted) return null;

  return (
    <>
      <TableRow key={user.userId} className="odd:bg-white even:bg-gray-50">
        <TableCell>
          <Avatar className="w-9 h-9">
            <AvatarImage src={user.image} alt={user.name} />
            <AvatarFallback className="bg-primary text-white">
              {user.name[0].toLocaleUpperCase()}
            </AvatarFallback>
          </Avatar>
        </TableCell>
        <TableCell>{user.name}</TableCell>
        <TableCell>{user.email}</TableCell>
        <TableCell
          className={cn(user.role.includes("admin") && "text-green-600")}
        >
          {user.role}
        </TableCell>
        <TableCell className="space-x-2">
          <Button
            size="icon"
            variant="outline"
            onClick={() => openEditModal(user)}
          >
            <Edit />
          </Button>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button size="icon" variant="outline" className="text-red-600">
                <Trash2 />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogTitle>Delete User</AlertDialogTitle>
              <p className="text-sm">
                Are you sure you want to delete this user?
              </p>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction onClick={() => handleDelete(user.userId)}>
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </TableCell>
      </TableRow>

      {/* Edit User Modal */}
      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit User</DialogTitle>
          </DialogHeader>
          <div className="mt-4 space-y-4">
            <label className="block text-sm font-medium mb-1">Role</label>
            <Select
              value={editRole}
              onValueChange={(value) => setEditRole(value)}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select role" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="user">User</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button onClick={handleUpdate}>Save changes</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
};
