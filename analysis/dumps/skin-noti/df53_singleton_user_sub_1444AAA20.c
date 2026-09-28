// singleton_user_sub_1444AAA20

char __fastcall sub_1444AAA20(__int64 a1)
{
  int v2; // ebx
  int v3; // eax
  __int64 v4; // rcx
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  _BYTE v8[32]; // [rsp+48h] [rbp-20h] BYREF

  sub_145F6F4B0(a1);
  sub_1467A9F20(a1, 0);
  sub_1467A9250(a1, 3);
  sub_1467AA1E0(a1);
  v2 = 0;
  *(_DWORD *)(a1 + 1752) = 0;
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1536) + 16LL))(*(_QWORD *)(a1 + 1536), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1552) + 16LL))(*(_QWORD *)(a1 + 1552), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1568) + 16LL))(*(_QWORD *)(a1 + 1568), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1584) + 16LL))(*(_QWORD *)(a1 + 1584), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1600) + 16LL))(*(_QWORD *)(a1 + 1600), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1616) + 16LL))(*(_QWORD *)(a1 + 1616), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1632) + 16LL))(*(_QWORD *)(a1 + 1632), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1648) + 16LL))(*(_QWORD *)(a1 + 1648), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1664) + 16LL))(*(_QWORD *)(a1 + 1664), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1680) + 16LL))(*(_QWORD *)(a1 + 1680), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1696) + 16LL))(*(_QWORD *)(a1 + 1696), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1712) + 16LL))(*(_QWORD *)(a1 + 1712), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1728) + 16LL))(*(_QWORD *)(a1 + 1728), 0);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1520) + 16LL))(*(_QWORD *)(a1 + 1520), 0);
  sub_146E9FBD0(a1 + 1760, 0, 0);
  v3 = sub_146E8C7D0(&unk_14A303080);
  sub_145A31380(v3, -1, 0, 0, -1, -1, 0);
  v4 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v5 = sub_146E8BA20(496);
    if ( v5 )
      v6 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v5);
    else
      v6 = 0;
    qword_14E659EA8 = (__int64)v6;
    (**v6)(v6);
    v4 = qword_14E659EA8;
  }
  LOBYTE(v2) = *(_BYTE *)sub_14449E0B0(v4, v8) != 0;
  *(_DWORD *)(a1 + 1756) = v2;
  sub_146EECBB0(*(_QWORD *)(a1 + 1568), 3 - (unsigned int)(unsigned __int8)v2);
  return 1;
}

